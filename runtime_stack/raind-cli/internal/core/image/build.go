package image

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	httpclient "raind/internal/core/client"
	"strings"
)

func NewServiceImageBuild() *ServiceImageBuild {
	return &ServiceImageBuild{}
}

type ServiceImageBuild struct{}

func (s *ServiceImageBuild) Build(param ServiceImageBuildModel) error {
	if param.ContextDir == "" {
		return fmt.Errorf("context directory is required")
	}
	if param.Tag == "" {
		return fmt.Errorf("image tag is required")
	}
	if param.Dripfile == "" {
		param.Dripfile = "Dripfile"
	}

	info, err := os.Stat(param.ContextDir)
	if err != nil {
		return fmt.Errorf("stat context dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("context path is not a directory: %s", param.ContextDir)
	}
	if _, err := os.Stat(filepath.Join(param.ContextDir, param.Dripfile)); err != nil {
		return fmt.Errorf("dripfile not found: %w", err)
	}

	tarPath := filepath.Join(os.TempDir(), sanitizeTarName(param.Tag)+".tar")
	if err := createTarFromDir(tarPath, param.ContextDir); err != nil {
		_ = os.Remove(tarPath)
		return err
	}
	defer os.Remove(tarPath)

	tarFile, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open tar: %w", err)
	}
	defer tarFile.Close()

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}

	query := url.Values{}
	query.Set("tag", param.Tag)
	query.Set("dripfile", param.Dripfile)
	endpoint := httpClient.BaseUrl + "/v1/images/build?" + query.Encode()

	req, err := http.NewRequest(http.MethodPost, endpoint, tarFile)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-tar")

	resp, err := httpClient.Client.Do(req)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ImageBuildResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := jsonDecode(resp.Body, &respModel); decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}

	if err := jsonDecode(resp.Body, &respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func sanitizeTarName(tag string) string {
	replacer := strings.NewReplacer("/", "_", ":", "_")
	return replacer.Replace(tag)
}

func createTarFromDir(tarPath, srcDir string) error {
	f, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("create tar: %w", err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		var linkTarget string
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}

		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}

		return nil
	})
}

func jsonDecode(r io.Reader, v interface{}) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
