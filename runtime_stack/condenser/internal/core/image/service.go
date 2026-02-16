package image

import (
	"condenser/internal/registry"
	"condenser/internal/registry/dockerhub"
	"condenser/internal/store/ilm"
	"condenser/internal/utils"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func NewImageService() *ImageService {
	return &ImageService{
		filesystemHandler: utils.NewFilesystemExecutor(),
		registryHandler:   dockerhub.NewRegistryDockerHub(),
		ilmHandler:        ilm.NewIlmManager(ilm.NewIlmStore(utils.IlmStorePath)),
	}
}

type ImageService struct {
	filesystemHandler utils.FilesystemHandler
	registryHandler   registry.RegistryHandler
	ilmHandler        ilm.IlmHandler
}

type singleManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"layers"`
}

func (s *ImageService) Pull(pullParameter ServicePullModel) error {
	targetOs := pullParameter.Os
	if targetOs == "" {
		targetOs = utils.HostOs()
	}
	targetArch := pullParameter.Arch
	if targetArch == "" {
		hostArch, err := utils.HostArch()
		if err != nil {
			return err
		}
		targetArch = hostArch
	}

	// pull image
	repository, reference, bundlePath, configPath, rootfsPath, err := s.registryHandler.PullImage(
		registry.RegistryPullModel{
			Image: pullParameter.Image,
			Os:    targetOs,
			Arch:  targetArch,
		},
	)
	if err != nil {
		return err
	}

	// add ilm entry
	if err := s.ilmHandler.StoreImage(
		repository, reference,
		bundlePath, configPath, rootfsPath,
	); err != nil {
		return err
	}

	return nil
}

func (s *ImageService) Remove(removeParameter ServiceRemoveModel) error {
	repo, ref, err := s.parseImageRef(removeParameter.Image)
	if err != nil {
		return err
	}

	// remove directory
	bundlePath, err := s.ilmHandler.GetBundlePath(repo, ref)
	if err != nil {
		return err
	}
	if err := s.filesystemHandler.RemoveAll(bundlePath); err != nil {
		return err
	}

	// remove ilm entry
	if err := s.ilmHandler.RemoveImage(repo, ref); err != nil {
		return err
	}

	return nil
}

func (s *ImageService) parseImageRef(imageStr string) (repository, reference string, err error) {
	// image string pattern
	// - ubuntu 				-> library/ubuntu:latest
	// - ubuntu:24.04 			-> library/ubuntu:24.04
	// - library/ubuntu:24.04 	-> library/ubuntu:24.04
	// - nginx@sha256:... 		-> library/nginx@sha256:...

	const defaultRegistry = "registry-1.docker.io"
	var (
		repo     string
		ref      string
		registry string
	)
	if strings.Contains(imageStr, "@") {
		parts := strings.SplitN(imageStr, "@", 2)
		repo = parts[0]
		ref = parts[1]
	} else {
		parts := strings.SplitN(imageStr, ":", 2)
		repo = parts[0]
		if len(parts) == 2 && parts[1] != "" {
			ref = parts[1]
		} else {
			ref = "latest"
		}
	}

	if repo == "" {
		return "", "", errors.New("empty repository")
	}

	if strings.Contains(repo, "/") {
		first := strings.SplitN(repo, "/", 2)[0]
		if isRegistryHost(first) {
			registry = normalizeRegistry(first)
			repo = strings.SplitN(repo, "/", 2)[1]
		}
	}

	if registry == "" || registry == defaultRegistry {
		if !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}
	} else {
		repo = registry + "/" + repo
	}
	return repo, ref, nil
}

func (s *ImageService) GetImageConfig(filepath string) (ImageConfigFile, error) {
	b, err := s.filesystemHandler.ReadFile(filepath)
	if err != nil {
		return ImageConfigFile{}, err
	}

	var imageConfig ImageConfigFile
	if err := json.Unmarshal(b, &imageConfig); err != nil {
		return ImageConfigFile{}, err
	}
	return imageConfig, nil
}

func (s *ImageService) GetImageList() ([]ImageInfo, error) {
	imageList, err := s.ilmHandler.GetImageList()
	if err != nil {
		return nil, err
	}

	var imageInfo []ImageInfo
	for _, il := range imageList {
		imageInfo = append(imageInfo, ImageInfo{
			Repository: il.Repository,
			Reference:  il.Reference,
			CreatedAt:  il.CreatedAt,
		})
	}

	return imageInfo, nil
}

func (s *ImageService) GetImageStatus(imageStr string) (ImageStatusInfo, error) {
	repo, ref, err := s.parseImageRef(imageStr)
	if err != nil {
		return ImageStatusInfo{}, err
	}

	info, err := s.ilmHandler.GetImageInfo(repo, ref)
	if err != nil {
		return ImageStatusInfo{}, err
	}

	user := ""
	if configPath, err := s.ilmHandler.GetConfigPath(repo, ref); err == nil {
		if cfg, err := s.GetImageConfig(configPath); err == nil {
			user = cfg.Config.User
		}
	}

	bundlePath, err := s.ilmHandler.GetBundlePath(repo, ref)
	if err != nil {
		return ImageStatusInfo{}, err
	}

	manifestBytes, err := s.readManifest(bundlePath)
	if err != nil {
		if s.filesystemHandler.IsNotExist(err) {
			_ = s.ilmHandler.RemoveImage(repo, ref)
			_ = s.filesystemHandler.RemoveAll(bundlePath)
			return ImageStatusInfo{}, fmt.Errorf("%s:%s not found", repo, ref)
		}
		return ImageStatusInfo{}, err
	}

	var m singleManifest
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		return ImageStatusInfo{}, err
	}

	var sizeBytes int64
	sizeBytes += m.Config.Size
	for _, l := range m.Layers {
		sizeBytes += l.Size
	}

	manifestDigest := sha256.Sum256(manifestBytes)
	manifestDigestStr := "sha256:" + hex.EncodeToString(manifestDigest[:])

	var repoTags []string
	if !strings.HasPrefix(ref, "sha256:") {
		repoTags = []string{repo + ":" + ref}
	}

	repoDigests := []string{repo + "@" + manifestDigestStr}

	return ImageStatusInfo{
		Repository:  info.Repository,
		Reference:   info.Reference,
		Id:          m.Config.Digest,
		RepoTags:    repoTags,
		RepoDigests: repoDigests,
		SizeBytes:   sizeBytes,
		CreatedAt:   info.CreatedAt,
		User:        user,
	}, nil
}

func (s *ImageService) GetImageFsInfo(imageStr string) (ImageFsInfo, error) {
	repo, ref, err := s.parseImageRef(imageStr)
	if err != nil {
		return ImageFsInfo{}, err
	}

	bundlePath, err := s.ilmHandler.GetBundlePath(repo, ref)
	if err != nil {
		return ImageFsInfo{}, err
	}

	usedBytes, err := s.dirSize(bundlePath)
	if err != nil {
		return ImageFsInfo{}, err
	}

	return ImageFsInfo{
		Image:     repo + ":" + ref,
		UsedBytes: usedBytes,
	}, nil
}

func (s *ImageService) readManifest(bundlePath string) ([]byte, error) {
	manifestSelected := filepath.Join(bundlePath, "manifest.selected.json")
	b, err := s.filesystemHandler.ReadFile(manifestSelected)
	if err == nil {
		return b, nil
	}
	if !s.filesystemHandler.IsNotExist(err) {
		return nil, err
	}
	manifestPath := filepath.Join(bundlePath, "manifest.json")
	return s.filesystemHandler.ReadFile(manifestPath)
}

func (s *ImageService) dirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("calc dir size failed: %w", err)
	}
	return total, nil
}

func isRegistryHost(host string) bool {
	if host == "localhost" {
		return true
	}
	return strings.Contains(host, ".") || strings.Contains(host, ":")
}

func normalizeRegistry(reg string) string {
	switch reg {
	case "docker.io", "index.docker.io":
		return "registry-1.docker.io"
	default:
		return reg
	}
}
