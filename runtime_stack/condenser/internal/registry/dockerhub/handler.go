package dockerhub

import (
	"archive/tar"
	"compress/gzip"
	"condenser/internal/registry"
	"condenser/internal/utils"
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
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const defaultRegistry = "registry-1.docker.io"

func NewRegistryDockerHub() *RegistryDockerHub {
	return &RegistryDockerHub{}
}

type RegistryDockerHub struct{}

func (s *RegistryDockerHub) PullImage(pullParameter registry.RegistryPullModel) (repository, reference, bundlePath, configPath, rootfsPath string, err error) {
	// 1. parse Image Reference
	imageRef, err := s.parseImageRef(pullParameter.Image)
	if err != nil {
		return "", "", "", "", "", err
	}

	// 2. create output directory
	storeRepo := s.storeRepository(imageRef)
	repoOut := filepath.Join(utils.LayerRootDir, storeRepo, imageRef.reference)
	if err := s.createOutputDirectory(repoOut); err != nil {
		return "", "", "", "", "", err
	}

	ctx := context.Background()
	httpClient := &http.Client{Timeout: 60 * time.Second}

	// 3. get Bearer Challenge
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
	m, err := s.parseSingleManifest(manifestBytes)
	if err != nil {
		if err := s.removeOutputDirectory(repoOut); err != nil {
			return "", "", "", "", "", err
		}
		return "", "", "", "", "", err
	}

	// 8. download blob
	if err := s.downloadBlobVerified(
		ctx, httpClient, imageRef, token,
		m.Config.Digest, filepath.Join(repoOut, "blobs", s.digestToFilename(m.Config.Digest)),
	); err != nil {
		if err := s.removeOutputDirectory(repoOut); err != nil {
			return "", "", "", "", "", err
		}
		return "", "", "", "", "", err
	}

	// 9. download layers
	for _, l := range m.Layers {
		if err := s.downloadBlobVerified(
			ctx, httpClient, imageRef, token,
			l.Digest, filepath.Join(repoOut, "blobs", s.digestToFilename(l.Digest)),
		); err != nil {
			if err := s.removeOutputDirectory(repoOut); err != nil {
				return "", "", "", "", "", err
			}
			return "", "", "", "", "", err
		}
	}

	// 10. create config.json
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
	if err := s.applyLayers(rootfsPath, layerPaths); err != nil {
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

func (s *RegistryDockerHub) downloadBlobVerified(ctx context.Context, client *http.Client, ref imageRefParts, token, digest, dest string) error {
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
	tee := io.TeeReader(resp.Body, h)

	if _, err := io.Copy(f, tee); err != nil {
		return err
	}

	sum := hex.EncodeToString(h.Sum(nil))
	want := strings.TrimPrefix(digest, "sha256:")
	if sum != want {
		return fmt.Errorf("digest mismatch: want %s got %s", want, sum)
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

func (s *RegistryDockerHub) applyLayers(rootfs string, layerBlobPaths []string) error {
	for i, p := range layerBlobPaths {
		if err := s.applyOneLayer(rootfs, p); err != nil {
			return fmt.Errorf("apply layer %d (%s): %w", i, p, err)
		}
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

		// remove /
		name := strings.TrimPrefix(hdr.Name, "/")
		name = filepath.Clean(name)
		if name == "." {
			continue
		}

		// protect path traversal
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
			if err := s.removeAllChildren(opaqueDir); err != nil {
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
			_ = os.RemoveAll(targetAbs)
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dstPath, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
			_ = os.Chtimes(dstPath, time.Now(), hdr.ModTime)
			_ = s.applyOwner(dstPath, hdr, false)

		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return err
			}
			if err := s.writeFileFromTar(dstPath, tr, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
			_ = os.Chtimes(dstPath, time.Now(), hdr.ModTime)
			_ = s.applyOwner(dstPath, hdr, false)

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return err
			}
			_ = os.RemoveAll(dstPath)
			if err := os.Symlink(hdr.Linkname, dstPath); err != nil {
				return err
			}
			_ = s.applyOwner(dstPath, hdr, true)

		case tar.TypeLink: // hardlink
			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return err
			}
			linkTarget := strings.TrimPrefix(hdr.Linkname, "/")
			linkTarget = filepath.Clean(linkTarget)
			targetAbs, err := s.joinRoot(rootfs, linkTarget)
			if err != nil {
				return err
			}
			_ = os.RemoveAll(dstPath)
			if err := os.Link(targetAbs, dstPath); err != nil {
				return fmt.Errorf("hardlink %s -> %s: %w", dstPath, targetAbs, err)
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
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
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

func (s *RegistryDockerHub) joinRoot(rootfs, rel string) (string, error) {
	rel = strings.TrimPrefix(rel, "/")
	rel = filepath.Clean(rel)
	if rel == "." {
		return rootfs, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %s", rel)
	}
	return filepath.Join(rootfs, rel), nil
}

func (s *RegistryDockerHub) removeAllChildren(dir string) error {
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dir, 0o755)
		}
		return err
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
		if err := os.RemoveAll(p); err != nil {
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
