package dockerhub

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"raind/internal/condenser/registry"
	"raind/internal/condenser/utils"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const defaultRegistry = "registry-1.docker.io"

func NewRegistryDockerHub() *RegistryDockerHub {
	return &RegistryDockerHub{}
}

type RegistryDockerHub struct{}

type progressReader struct {
	reader   io.Reader
	total    int64
	current  int64
	lastEmit int64
	emit     func(current, total int64)
}

func newProgressReader(reader io.Reader, total int64, emit func(current, total int64)) *progressReader {
	return &progressReader{
		reader: reader,
		total:  total,
		emit:   emit,
	}
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.current += int64(n)
		if r.current-r.lastEmit >= 512*1024 || r.current == r.total {
			r.lastEmit = r.current
			r.emit(r.current, r.total)
		}
	}
	return n, err
}

func shortDigest(digest string) string {
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) <= 12 {
		return digest
	}
	return digest[:12]
}

func (s *RegistryDockerHub) PullImage(pullParameter registry.RegistryPullModel) (repository, reference, bundlePath, configPath, rootfsPath string, err error) {
	emit := func(status, id, detail string, current, total int64) {
		if pullParameter.Progress == nil {
			return
		}
		pullParameter.Progress(registry.ProgressEvent{
			Status:  status,
			ID:      id,
			Detail:  detail,
			Current: current,
			Total:   total,
		})
	}

	// 1. parse Image Reference
	imageRef, err := s.parseImageRef(pullParameter.Image)
	if err != nil {
		return "", "", "", "", "", err
	}
	imageID := imageRef.repository + ":" + imageRef.reference
	emit("resolving", imageID, "resolving image reference", 0, 0)

	// 2. create output directory
	storeRepo := s.storeRepository(imageRef)
	repoOut := filepath.Join(utils.LayerRootDir, storeRepo, imageRef.reference)
	if err := s.createOutputDirectory(repoOut); err != nil {
		return "", "", "", "", "", err
	}

	ctx := context.Background()
	httpClient := &http.Client{Timeout: 60 * time.Second}

	// 3. get Bearer Challenge
	emit("auth", imageRef.registry, "requesting registry authentication challenge", 0, 0)
	realm, service, err := s.getBearerChallenge(ctx, httpClient, imageRef.registry)
	if err != nil {
		if err := s.removeOutputDirectory(repoOut); err != nil {
			return "", "", "", "", "", err
		}
		return "", "", "", "", "", err
	}

	// 4. get token
	token := ""
	if realm != "" && service != "" {
		emit("auth", imageRef.registry, "requesting pull token", 0, 0)
		scope := fmt.Sprintf("repository:%s:pull", imageRef.repository)
		token, err = s.fetchToken(ctx, httpClient, realm, service, scope)
		if err != nil {
			if err := s.removeOutputDirectory(repoOut); err != nil {
				return "", "", "", "", "", err
			}
			return "", "", "", "", "", err
		}
	}

	// 5. get manifest (manifest list) and store .json
	emit("manifest", imageID, "fetching manifest", 0, 0)
	manifestBytes, mediaType, err := s.fetchManifest(ctx, httpClient, imageRef, token)
	if err != nil {
		if err := s.removeOutputDirectory(repoOut); err != nil {
			return "", "", "", "", "", err
		}
		return "", "", "", "", "", err
	}
	if err := s.storeManifest(repoOut, manifestBytes, "manifest.json"); err != nil {
		if err := s.removeOutputDirectory(repoOut); err != nil {
			return "", "", "", "", "", err
		}
		return "", "", "", "", "", err
	}

	// 6. get manifest if the mediaType is list
	if s.isManifestListMediaType(mediaType) {
		emit("manifest", imageID, "selecting platform manifest "+pullParameter.Os+"/"+pullParameter.Arch, 0, 0)
		// pick digest from manifest list
		dgst, err := s.pickFromManifestList(manifestBytes, pullParameter.Os, pullParameter.Arch)
		if err != nil {
			if err := s.removeOutputDirectory(repoOut); err != nil {
				return "", "", "", "", "", err
			}
			return "", "", "", "", "", err
		}
		imageRef2 := imageRef
		imageRef2.reference = dgst // set digest to reference
		emit("manifest", dgst, "fetching selected manifest", 0, 0)
		manifestBytes, mediaType, err = s.fetchManifest(ctx, httpClient, imageRef2, token)
		if err != nil {
			if err := s.removeOutputDirectory(repoOut); err != nil {
				return "", "", "", "", "", err
			}
			return "", "", "", "", "", err
		}
		if err := s.storeManifest(repoOut, manifestBytes, "manifest.selected.json"); err != nil {
			if err := s.removeOutputDirectory(repoOut); err != nil {
				return "", "", "", "", "", err
			}
			return "", "", "", "", "", err
		}
	}

	// 7. parse manifest
	emit("manifest", imageID, "parsing manifest", 0, 0)
	m, err := s.parseSingleManifest(manifestBytes)
	if err != nil {
		if err := s.removeOutputDirectory(repoOut); err != nil {
			return "", "", "", "", "", err
		}
		return "", "", "", "", "", err
	}

	// 8. download blob
	emit("config", m.Config.Digest, "downloading image config", 0, m.Config.Size)
	if err := s.downloadBlobVerified(
		ctx, httpClient, imageRef, token,
		m.Config.Digest, filepath.Join(repoOut, "blobs", s.digestToFilename(m.Config.Digest)),
		pullParameter.Progress,
	); err != nil {
		if err := s.removeOutputDirectory(repoOut); err != nil {
			return "", "", "", "", "", err
		}
		return "", "", "", "", "", err
	}

	// 9. download layers
	for i, l := range m.Layers {
		emit("layer", shortDigest(l.Digest), fmt.Sprintf("downloading layer %d/%d", i+1, len(m.Layers)), 0, l.Size)
		if err := s.downloadBlobVerified(
			ctx, httpClient, imageRef, token,
			l.Digest, filepath.Join(repoOut, "blobs", s.digestToFilename(l.Digest)),
			pullParameter.Progress,
		); err != nil {
			if err := s.removeOutputDirectory(repoOut); err != nil {
				return "", "", "", "", "", err
			}
			return "", "", "", "", "", err
		}
	}

	// 10. create config.json
	emit("config", m.Config.Digest, "writing image config", 0, 0)
	configPath = filepath.Join(repoOut, "config.json")
	if err := s.copyFile(
		filepath.Join(repoOut, "blobs", s.digestToFilename(m.Config.Digest)),
		configPath,
	); err != nil {
		if err := s.removeOutputDirectory(repoOut); err != nil {
			return "", "", "", "", "", err
		}
		return "", "", "", "", "", err
	}

	// 11. extract layer
	rootfsPath = filepath.Join(repoOut, "rootfs")
	var layerPaths []string
	for _, l := range m.Layers {
		p := filepath.Join(repoOut, "blobs", s.digestToFilename(l.Digest))
		layerPaths = append(layerPaths, p)
	}
	if err := s.applyLayers(rootfsPath, layerPaths, m.Layers, pullParameter.Progress); err != nil {
		if err := s.removeOutputDirectory(repoOut); err != nil {
			return "", "", "", "", "", err
		}
		return "", "", "", "", "", err
	}

	return s.storeRepository(imageRef), imageRef.reference, repoOut, configPath, rootfsPath, nil
}

func (s *RegistryDockerHub) createOutputDirectory(repoOut string) error {
	// image root
	if err := os.MkdirAll(repoOut, 0o755); err != nil {
		return err
	}
	// blob
	if err := os.MkdirAll(filepath.Join(repoOut, "blobs"), 0o755); err != nil {
		return err
	}
	// rootfs
	if err := os.MkdirAll(filepath.Join(repoOut, "rootfs"), 0o755); err != nil {
		return err
	}
	return nil
}

func (s *RegistryDockerHub) removeOutputDirectory(repoOut string) error {
	if err := os.RemoveAll(repoOut); err != nil {
		return err
	}
	return nil
}

func (s *RegistryDockerHub) parseImageRef(imageStr string) (imageRefParts, error) {
	// image string pattern
	// - ubuntu 				-> library/ubuntu:latest
	// - ubuntu:24.04 			-> library/ubuntu:24.04
	// - library/ubuntu:24.04 	-> library/ubuntu:24.04
	// - nginx@sha256:... 		-> library/nginx@sha256:...
	// - registry.k8s.io/kube-apiserver:v1.32.11
	// - localhost:5000/app:latest

	var name, repo, ref string
	if strings.Contains(imageStr, "@") {
		parts := strings.SplitN(imageStr, "@", 2)
		name, ref = parts[0], parts[1]
	} else {
		name = imageStr
		lastColon := strings.LastIndex(name, ":")
		lastSlash := strings.LastIndex(name, "/")
		if lastColon > lastSlash {
			ref = name[lastColon+1:]
			name = name[:lastColon]
		} else {
			ref = "latest"
		}
	}

	if name == "" {
		return imageRefParts{}, errors.New("empty repository")
	}

	reg := defaultRegistry
	repo = name
	parts := strings.Split(name, "/")
	if len(parts) > 1 && s.isRegistryHost(parts[0]) {
		reg = s.normalizeRegistry(parts[0])
		repo = strings.Join(parts[1:], "/")
	} else if len(parts) == 1 {
		repo = "library/" + name
	}

	return imageRefParts{
		registry:   reg,
		repository: repo,
		reference:  ref,
	}, nil
}

func (s *RegistryDockerHub) getBearerChallenge(ctx context.Context, client *http.Client, registry string) (realm, service string, err error) {
	u := "https://" + registry + "/v2/"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	// http request
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	// validate status
	if resp.StatusCode == http.StatusOK {
		return "", "", nil
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return "", "", fmt.Errorf("expected 401 from /v2/, got %d", resp.StatusCode)
	}

	// helper: parse www-authenticate
	parseWwwAuthenticate := func(h string) (realm, service string, err error) {
		h = strings.TrimSpace(h)
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			return "", "", fmt.Errorf("unexpected Www-Authenticate: %s", h)
		}
		rest := strings.TrimSpace(h[len("Bearer "):])
		// retrieve key
		parts := s.splitCommaPreserveQuotes(rest)
		kv := map[string]string{}
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			k, v, ok := strings.Cut(p, "=")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			v = strings.Trim(strings.TrimSpace(v), `"`)
			kv[k] = v
		}
		realm = kv["realm"]
		service = kv["service"]
		if realm == "" || service == "" {
			return "", "", fmt.Errorf("failed to parse bearer challenge: %s", h)
		}
		return realm, service, nil
	}
	h := resp.Header.Get("Www-Authenticate")
	// e.g. Bearer realm="https://auth.docker.io/token",service="registry.docker.io"scope="..."
	realm, service, err = parseWwwAuthenticate(h)
	if err != nil {
		return "", "", err
	}
	return realm, service, nil
}

func (s *RegistryDockerHub) splitCommaPreserveQuotes(str string) []string {
	var out []string
	var cur strings.Builder
	inQ := false
	for _, r := range str {
		switch r {
		case '"':
			inQ = !inQ
			cur.WriteRune(r)
		case ',':
			if inQ {
				cur.WriteRune(r)
			} else {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func (s *RegistryDockerHub) fetchToken(ctx context.Context, client *http.Client, realm, service, scope string) (string, error) {
	u, err := url.Parse(realm)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("service", service)
	q.Set("scope", scope)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: %d: %s", resp.StatusCode, string(b))
	}

	var tr tokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.Token == "" {
		return "", errors.New("no token in response")
	}
	return tr.AccessToken, nil
}

func (s *RegistryDockerHub) fetchManifest(ctx context.Context, client *http.Client, ref imageRefParts, token string) (body []byte, mediaType string, err error) {
	u := fmt.Sprintf("https://%s/v2/%s/manifests/%s", ref.registry, ref.repository, ref.reference)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
	}, ", "))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("manifest fetch failed: %d: %s", resp.StatusCode, string(b))
	}
	mediaType = resp.Header.Get("Content-Type")
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return b, mediaType, nil
}

func (s *RegistryDockerHub) storeManifest(repoOut string, data []byte, filename string) error {
	if err := os.WriteFile(filepath.Join(repoOut, filename), data, 0o644); err != nil {
		return err
	}
	return nil
}

func (s *RegistryDockerHub) isManifestListMediaType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	return ct == "application/vnd.docker.distribution.manifest.list.v2+json" ||
		ct == "application/vnd.oci.image.index.v1+json"
}

func (s *RegistryDockerHub) pickFromManifestList(b []byte, targetOs, targetArch string) (string, error) {
	var ml manifestList
	if err := json.Unmarshal(b, &ml); err != nil {
		return "", err
	}
	for _, m := range ml.Manifests {
		if m.Platform.OS == targetOs && m.Platform.Architecture == targetArch {
			if m.Digest == "" {
				continue
			}
			return m.Digest, nil
		}
	}
	return "", fmt.Errorf("no manifest for platform %s/%s", targetOs, targetArch)
}

func (s *RegistryDockerHub) parseSingleManifest(b []byte) (*singleManifest, error) {
	var m singleManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.Config.Digest == "" || len(m.Layers) == 0 {
		return nil, errors.New("unexpected manifest (no config/layers)")
	}
	return &m, nil
}

func (s *RegistryDockerHub) digestToFilename(d string) string {
	// sha256:abcd... -> sha256_abcd...
	return strings.ReplaceAll(d, ":", "_")
}

func (s *RegistryDockerHub) downloadBlobVerified(ctx context.Context, client *http.Client, ref imageRefParts, token, digest, dest string, progress registry.ProgressFunc) error {
	if !strings.HasPrefix(digest, "sha256:") {
		return fmt.Errorf("only sha256 digest supported: %s", digest)
	}
	u := fmt.Sprintf("https://%s/v2/%s/blobs/%s", ref.registry, ref.repository, digest)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("blob fetch failed: %d: %s", resp.StatusCode, string(b))
	}

	// store
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	reader := io.Reader(resp.Body)
	if progress != nil {
		progress(registry.ProgressEvent{
			Status: "downloading",
			ID:     shortDigest(digest),
			Detail: "downloading",
			Total:  resp.ContentLength,
		})
		reader = newProgressReader(resp.Body, resp.ContentLength, func(current, total int64) {
			progress(registry.ProgressEvent{
				Status:  "downloading",
				ID:      shortDigest(digest),
				Detail:  "downloading",
				Current: current,
				Total:   total,
			})
		})
	}
	tee := io.TeeReader(reader, h)

	if _, err := io.Copy(f, tee); err != nil {
		return err
	}

	sum := hex.EncodeToString(h.Sum(nil))
	want := strings.TrimPrefix(digest, "sha256:")
	if sum != want {
		return fmt.Errorf("digest mismatch: want %s got %s", want, sum)
	}
	if progress != nil {
		progress(registry.ProgressEvent{
			Status:  "complete",
			ID:      shortDigest(digest),
			Detail:  "download complete",
			Current: resp.ContentLength,
			Total:   resp.ContentLength,
		})
	}
	return nil
}

func (s *RegistryDockerHub) copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func (s *RegistryDockerHub) applyLayers(rootfs string, layerBlobPaths []string, layers []struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}, progress registry.ProgressFunc) error {
	for i, p := range layerBlobPaths {
		if progress != nil {
			progress(registry.ProgressEvent{
				Status:  "extracting",
				ID:      shortDigest(layers[i].Digest),
				Detail:  fmt.Sprintf("extracting layer %d/%d", i+1, len(layerBlobPaths)),
				Current: int64(i),
				Total:   int64(len(layerBlobPaths)),
			})
		}
		if err := s.applyOneLayer(rootfs, p); err != nil {
			return fmt.Errorf("apply layer %d (%s): %w", i, p, err)
		}
	}
	if progress != nil {
		progress(registry.ProgressEvent{
			Status:  "extracting",
			Detail:  "extract complete",
			Current: int64(len(layerBlobPaths)),
			Total:   int64(len(layerBlobPaths)),
		})
	}
	return nil
}

func (s *RegistryDockerHub) applyOneLayer(rootfs, layerBlobPath string) error {
	f, err := os.Open(layerBlobPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}
		if hdr == nil {
			continue
		}

		name, err := cleanArchivePath(hdr.Name)
		if err != nil {
			return fmt.Errorf("invalid path %q: %w", hdr.Name, err)
		}
		if name == "" {
			continue
		}

		dstPath, err := s.joinRoot(rootfs, name)
		if err != nil {
			return fmt.Errorf("invalid path %q: %w", hdr.Name, err)
		}

		// whiteout
		base := filepath.Base(name)
		dir := filepath.Dir(name)

		if base == ".wh..wh..opq" {
			opaqueDir, err := s.joinRoot(rootfs, dir)
			if err != nil {
				return err
			}
			if err := s.removeAllChildren(rootfs, opaqueDir); err != nil {
				return fmt.Errorf("opaque dir cleanup %s: %w", opaqueDir, err)
			}
			continue
		}

		if strings.HasPrefix(base, ".wh.") {
			targetName := strings.TrimPrefix(base, ".wh.")
			targetRel := filepath.Join(dir, targetName)
			targetAbs, err := s.joinRoot(rootfs, targetRel)
			if err != nil {
				return err
			}
			if err := s.removeArchiveTarget(rootfs, targetAbs); err != nil {
				return err
			}
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := s.ensureSafeExtractionDir(rootfs, dstPath, os.FileMode(hdr.Mode).Perm()); err != nil {
				return err
			}
			_ = os.Chtimes(dstPath, time.Now(), hdr.ModTime)
			_ = s.applyOwner(dstPath, hdr, false)

		case tar.TypeReg, tar.TypeRegA:
			if err := s.ensureSafeParentDir(rootfs, dstPath); err != nil {
				return err
			}
			if err := s.writeFileFromTar(dstPath, tr, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
			_ = os.Chtimes(dstPath, time.Now(), hdr.ModTime)
			_ = s.applyOwner(dstPath, hdr, false)

		case tar.TypeSymlink:
			if err := s.ensureSafeParentDir(rootfs, dstPath); err != nil {
				return err
			}
			if err := validateImageLayerSymlinkTarget(rootfs, dstPath, hdr.Linkname); err != nil {
				return err
			}
			if err := s.removeArchiveTarget(rootfs, dstPath); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, dstPath); err != nil {
				return err
			}
			_ = s.applyOwner(dstPath, hdr, true)

		case tar.TypeLink: // hardlink
			if err := s.ensureSafeParentDir(rootfs, dstPath); err != nil {
				return err
			}
			linkTarget, err := cleanArchivePath(hdr.Linkname)
			if err != nil {
				return fmt.Errorf("invalid hardlink target %q: %w", hdr.Linkname, err)
			}
			if linkTarget == "" {
				return fmt.Errorf("invalid hardlink target %q", hdr.Linkname)
			}
			targetAbs, err := s.joinRoot(rootfs, linkTarget)
			if err != nil {
				return err
			}
			realTarget, err := s.resolveExistingPathUnderRoot(rootfs, targetAbs)
			if err != nil {
				return err
			}
			st, err := os.Stat(realTarget)
			if err != nil {
				return err
			}
			if !st.Mode().IsRegular() {
				return fmt.Errorf("hardlink target is not a regular file: %s", hdr.Linkname)
			}
			if err := s.removeArchiveTarget(rootfs, dstPath); err != nil {
				return err
			}
			if err := os.Link(realTarget, dstPath); err != nil {
				return fmt.Errorf("hardlink %s -> %s: %w", dstPath, realTarget, err)
			}
			_ = s.applyOwner(dstPath, hdr, false)

		case tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			if strings.HasPrefix(name, "dev/") {
				continue
			}
			return fmt.Errorf("special file not supported: typefalg %v for %s", hdr.Typeflag, hdr.Name)

		default:
			return fmt.Errorf("unsupported tar typeflag %v for %s", hdr.Typeflag, hdr.Name)
		}
	}
}

func (s *RegistryDockerHub) applyOwner(path string, hdr *tar.Header, isSymlink bool) error {
	uid, gid := hdr.Uid, hdr.Gid

	if isSymlink {
		if err := unix.Lchown(path, uid, gid); err != nil {
			return err
		}
		return nil
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	return nil
}

func (s *RegistryDockerHub) writeFileFromTar(dstPath string, r io.Reader, mode os.FileMode) error {
	tmp := dstPath + ".tmp"
	fd, err := unix.Open(tmp, unix.O_CREAT|unix.O_WRONLY|unix.O_TRUNC|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode.Perm()))
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), tmp)
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	// atomic-ish swap
	_ = os.RemoveAll(dstPath)
	return os.Rename(tmp, dstPath)
}

func cleanArchivePath(name string) (string, error) {
	if strings.ContainsRune(name, '\x00') {
		return "", fmt.Errorf("archive path contains NUL byte")
	}
	name = strings.TrimPrefix(name, "/")
	name = filepath.Clean(name)
	if name == "." || name == "" {
		return "", nil
	}
	if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive path escapes root: %s", name)
	}
	return name, nil
}

func validateImageLayerSymlinkTarget(rootfs, dstPath, linkname string) error {
	if linkname == "" {
		return fmt.Errorf("symlink target is empty")
	}
	if strings.ContainsRune(linkname, '\x00') {
		return fmt.Errorf("symlink target contains NUL byte")
	}

	absRoot, err := filepath.Abs(rootfs)
	if err != nil {
		return fmt.Errorf("resolve rootfs: %w", err)
	}
	absRoot = filepath.Clean(absRoot)

	var target string
	if filepath.IsAbs(linkname) {
		target = filepath.Join(absRoot, strings.TrimPrefix(filepath.Clean(linkname), string(filepath.Separator)))
	} else {
		target = filepath.Clean(filepath.Join(filepath.Dir(dstPath), linkname))
	}

	rel, err := filepath.Rel(absRoot, target)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("symlink target escapes rootfs: %s -> %s", dstPath, linkname)
	}
	return nil
}

func (s *RegistryDockerHub) ensureSafeParentDir(rootfs, target string) error {
	return s.ensureSafeExtractionDir(rootfs, filepath.Dir(target), 0o755)
}

func (s *RegistryDockerHub) ensureSafeExtractionDir(rootfs, dir string, perm os.FileMode) error {
	absRoot, err := filepath.Abs(rootfs)
	if err != nil {
		return fmt.Errorf("resolve rootfs: %w", err)
	}
	absRoot = filepath.Clean(absRoot)
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return fmt.Errorf("resolve real rootfs: %w", err)
	}
	realRoot = filepath.Clean(realRoot)

	dir, err = filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve extraction directory: %w", err)
	}
	dir = filepath.Clean(dir)

	rel, err := filepath.Rel(absRoot, dir)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("extraction directory escapes root: %s", dir)
	}
	if rel == "." {
		return nil
	}

	current := absRoot
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return fmt.Errorf("extraction directory escapes root: %s", dir)
		}

		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				resolved, err := filepath.EvalSymlinks(current)
				if err != nil {
					return fmt.Errorf("resolve symlink while extracting layer: %s: %w", current, err)
				}
				resolved = filepath.Clean(resolved)
				if err := ensureResolvedPathUnderRoot(realRoot, resolved); err != nil {
					return fmt.Errorf("symlink parent escapes rootfs while extracting layer: %s: %w", current, err)
				}
				st, err := os.Stat(resolved)
				if err != nil {
					return err
				}
				if !st.IsDir() {
					return fmt.Errorf("not a directory while extracting layer: %s", current)
				}
				current = resolved
				continue
			}
			if !info.IsDir() {
				return fmt.Errorf("not a directory while extracting layer: %s", current)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(current, perm); err != nil {
			if !os.IsExist(err) {
				return err
			}
			info, statErr := os.Lstat(current)
			if statErr != nil {
				return statErr
			}
			if info.Mode()&os.ModeSymlink != 0 {
				resolved, err := filepath.EvalSymlinks(current)
				if err != nil {
					return fmt.Errorf("resolve symlink while extracting layer: %s: %w", current, err)
				}
				resolved = filepath.Clean(resolved)
				if err := ensureResolvedPathUnderRoot(realRoot, resolved); err != nil {
					return fmt.Errorf("symlink parent escapes rootfs while extracting layer: %s: %w", current, err)
				}
				st, err := os.Stat(resolved)
				if err != nil {
					return err
				}
				if !st.IsDir() {
					return fmt.Errorf("not a directory while extracting layer: %s", current)
				}
				current = resolved
				continue
			}
			if !info.IsDir() {
				return fmt.Errorf("not a directory while extracting layer: %s", current)
			}
		}
	}
	return nil
}

func ensureResolvedPathUnderRoot(realRoot, resolved string) error {
	rel, err := filepath.Rel(realRoot, filepath.Clean(resolved))
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("resolved path escapes rootfs: %s", resolved)
	}
	return nil
}

func (s *RegistryDockerHub) removeArchiveTarget(rootfs, target string) error {
	if err := s.ensureSafeParentDir(rootfs, target); err != nil {
		return err
	}
	return os.RemoveAll(target)
}

func (s *RegistryDockerHub) resolveExistingPathUnderRoot(rootfs, target string) (string, error) {
	realRoot, err := filepath.EvalSymlinks(rootfs)
	if err != nil {
		return "", fmt.Errorf("resolve real rootfs: %w", err)
	}
	realRoot = filepath.Clean(realRoot)

	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("resolve path under rootfs: %w", err)
	}
	realTarget = filepath.Clean(realTarget)

	rel, err := filepath.Rel(realRoot, realTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("resolved path escapes rootfs: %s", target)
	}
	return realTarget, nil
}

func (s *RegistryDockerHub) joinRoot(rootfs, rel string) (string, error) {
	rel = strings.TrimPrefix(rel, "/")
	rel = filepath.Clean(rel)

	absRoot, err := filepath.Abs(rootfs)
	if err != nil {
		return "", fmt.Errorf("resolve rootfs: %w", err)
	}
	if rel == "." {
		return absRoot, nil
	}

	candidate := filepath.Join(absRoot, rel)
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve candidate path: %w", err)
	}

	relToRoot, err := filepath.Rel(absRoot, absCandidate)
	if err != nil {
		return "", fmt.Errorf("rel path check failed: %w", err)
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %s", rel)
	}

	return absCandidate, nil
}

func (s *RegistryDockerHub) removeAllChildren(rootfs, dir string) error {
	if err := s.ensureSafeExtractionDir(rootfs, dir, 0o755); err != nil {
		return err
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to clean symlink directory while extracting: %s", dir)
	}
	if !st.IsDir() {
		return fmt.Errorf("not a dir: %s", dir)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		p := filepath.Join(dir, e.Name())
		if err := s.removeArchiveTarget(rootfs, p); err != nil {
			return err
		}
	}
	return nil
}

func (s *RegistryDockerHub) isRegistryHost(host string) bool {
	if host == "localhost" {
		return true
	}
	return strings.Contains(host, ".") || strings.Contains(host, ":")
}

func (s *RegistryDockerHub) normalizeRegistry(reg string) string {
	switch reg {
	case "docker.io", "index.docker.io":
		return defaultRegistry
	default:
		return reg
	}
}

func (s *RegistryDockerHub) storeRepository(ref imageRefParts) string {
	if ref.registry == defaultRegistry {
		return ref.repository
	}
	return ref.registry + "/" + ref.repository
}
