package ilm

import (
	"condenser/internal/utils"
	"fmt"
	"os"
	"time"
)

func NewIlmManager(ilmStore *IlmStore) *IlmManager {
	return &IlmManager{
		ilmStore:          ilmStore,
		filesystemHandler: utils.NewFilesystemExecutor(),
	}
}

type IlmManager struct {
	ilmStore          *IlmStore
	filesystemHandler utils.FilesystemHandler
}

func (m *IlmManager) StoreImage(repository, reference, bundlePath, configPath, rootfsPath string) error {
	return m.ilmStore.withLock(func(st *ImageLayerState) error {
		if st.Repositories == nil {
			st.Repositories = map[string]RepositoryInfo{}
		}
		repoInfo, ok := st.Repositories[repository]
		if !ok {
			repoInfo = RepositoryInfo{
				References: map[string]ReferenceInfo{},
			}
		}
		if repoInfo.References == nil {
			repoInfo.References = map[string]ReferenceInfo{}
		}

		repoInfo.References[reference] = ReferenceInfo{
			BundlePath: bundlePath,
			ConfigPath: configPath,
			RootfsPath: rootfsPath,
			CreatedAt:  time.Now(),
		}

		st.Repositories[repository] = repoInfo
		return nil
	})
}

func (m *IlmManager) RemoveImage(repository string, reference string) error {
	return m.ilmStore.withLock(func(st *ImageLayerState) error {
		repo, ok := st.Repositories[repository]
		if !ok {
			return fmt.Errorf("%s:%s not found", repository, reference)
		}
		if _, ok := repo.References[reference]; !ok {
			return fmt.Errorf("%s:%s not found", repository, reference)
		}
		delete(st.Repositories[repository].References, reference)
		return nil
	})
}

func (s *IlmManager) GetBundlePath(repository string, reference string) (string, error) {
	var bundlePath string

	err := s.ilmStore.withRLock(func(st *ImageLayerState) error {
		bundlePath = st.Repositories[repository].References[reference].BundlePath
		if bundlePath == "" {
			return fmt.Errorf("bundle path not found.")
		}
		return nil
	})
	return bundlePath, err
}

func (s *IlmManager) GetConfigPath(repository string, reference string) (string, error) {
	var configPath string

	err := s.ilmStore.withRLock(func(st *ImageLayerState) error {
		configPath = st.Repositories[repository].References[reference].ConfigPath
		if configPath == "" {
			return fmt.Errorf("config path not found.")
		}
		return nil
	})
	return configPath, err
}

func (s *IlmManager) GetRootfsPath(repository string, reference string) (string, error) {
	var rootfsPath string

	err := s.ilmStore.withRLock(func(st *ImageLayerState) error {
		rootfsPath = st.Repositories[repository].References[reference].RootfsPath
		if rootfsPath == "" {
			return fmt.Errorf("bundle path not found.")
		}
		return nil
	})
	return rootfsPath, err
}

func (s *IlmManager) GetImageList() ([]ImageInfo, error) {
	var imageList []ImageInfo

	err := s.ilmStore.withRLock(func(st *ImageLayerState) error {
		for repo, refs := range st.Repositories {
			for ref, info := range refs.References {
				imageList = append(imageList, ImageInfo{
					Repository: repo,
					Reference:  ref,
					CreatedAt:  info.CreatedAt,
				})
			}
		}
		return nil
	})
	return imageList, err
}

func (s *IlmManager) GetImageInfo(repository string, reference string) (ImageInfo, error) {
	var info ImageInfo
	err := s.ilmStore.withRLock(func(st *ImageLayerState) error {
		repo, ok := st.Repositories[repository]
		if !ok {
			return fmt.Errorf("%s:%s not found", repository, reference)
		}
		refInfo, ok := repo.References[reference]
		if !ok {
			return fmt.Errorf("%s:%s not found", repository, reference)
		}
		info = ImageInfo{
			Repository: repository,
			Reference:  reference,
			CreatedAt:  refInfo.CreatedAt,
		}
		return nil
	})
	return info, err
}

func (s *IlmManager) IsImageExist(imageRepo, imageRef string) bool {
	var (
		found      bool
		configPath string
	)

	s.ilmStore.withRLock(func(st *ImageLayerState) error {
		for repo, refs := range st.Repositories {
			if repo != imageRepo {
				continue
			}
			for ref, info := range refs.References {
				if ref != imageRef {
					continue
				}
				found = true
				configPath = info.ConfigPath
				return nil
			}
		}
		found = false
		return nil
	})

	if !found {
		return false
	}
	if configPath == "" {
		return false
	}
	if _, err := os.Stat(configPath); err != nil {
		if s.filesystemHandler.IsNotExist(err) {
			return false
		}
		return false
	}
	return true
}
