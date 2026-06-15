package image

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"raind/internal/condenser/registry"
	"raind/internal/condenser/store/ilm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageServicePullStoresImageAfterRegistryPull(t *testing.T) {
	registryHandler := &fakeRegistryHandler{
		repository: "library/alpine",
		reference:  "latest",
		bundlePath: "/bundle",
		configPath: "/config.json",
		rootfsPath: "/rootfs",
	}
	ilmHandler := &fakeImageIlmHandler{}
	service := &ImageService{registryHandler: registryHandler, ilmHandler: ilmHandler}

	err := service.Pull(ServicePullModel{Image: "alpine"})

	require.NoError(t, err)
	assert.Equal(t, "alpine", registryHandler.request.Image)
	assert.Equal(t, storedImage{repository: "library/alpine", reference: "latest", bundlePath: "/bundle", configPath: "/config.json", rootfsPath: "/rootfs"}, ilmHandler.stored)
}

func TestImageServiceRemoveDeletesBundleAndStoreEntry(t *testing.T) {
	ilmHandler := &fakeImageIlmHandler{bundlePath: "/bundle", rootfsPath: "/bundle/rootfs"}
	fsHandler := &fakeImageFilesystemHandler{}
	service := &ImageService{filesystemHandler: fsHandler, ilmHandler: ilmHandler}

	err := service.Remove(ServiceRemoveModel{Image: "alpine:latest"})

	require.NoError(t, err)
	assert.Equal(t, []string{"/bundle/rootless-shifted", "/bundle"}, fsHandler.removedAll)
	assert.Equal(t, "library/alpine", ilmHandler.removedRepo)
	assert.Equal(t, "latest", ilmHandler.removedRef)
}

func TestImageServiceRemoveDeletesRootlessCacheBesideRootfs(t *testing.T) {
	ilmHandler := &fakeImageIlmHandler{bundlePath: "/image/bundle", rootfsPath: "/image/layers/local/app/latest/rootfs"}
	fsHandler := &fakeImageFilesystemHandler{}
	service := &ImageService{filesystemHandler: fsHandler, ilmHandler: ilmHandler}

	err := service.Remove(ServiceRemoveModel{Image: "local/app:latest"})

	require.NoError(t, err)
	assert.Equal(t, []string{
		"/image/bundle/rootless-shifted",
		"/image/layers/local/app/latest/rootless-shifted",
		"/image/bundle",
	}, fsHandler.removedAll)
}

func TestImageServiceStatusRemovesStaleEntryWhenManifestMissing(t *testing.T) {
	ilmHandler := &fakeImageIlmHandler{
		bundlePath: "/bundle",
		configPath: "/config.json",
		info:       ilm.ImageInfo{Repository: "library/alpine", Reference: "latest", CreatedAt: time.Now()},
	}
	fsHandler := &fakeImageFilesystemHandler{readErr: os.ErrNotExist}
	service := &ImageService{filesystemHandler: fsHandler, ilmHandler: ilmHandler}

	_, err := service.GetImageStatus("alpine:latest")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "library/alpine:latest not found")
	assert.Equal(t, "library/alpine", ilmHandler.removedRepo)
	assert.Equal(t, []string{"/bundle"}, fsHandler.removedAll)
}

func TestSafeJoinRejectsTarTraversal(t *testing.T) {
	_, err := safeJoin("/context", "../secret")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path")
}

type fakeRegistryHandler struct {
	request    registry.RegistryPullModel
	repository string
	reference  string
	bundlePath string
	configPath string
	rootfsPath string
	err        error
}

func (f *fakeRegistryHandler) PullImage(model registry.RegistryPullModel) (string, string, string, string, string, error) {
	f.request = model
	return f.repository, f.reference, f.bundlePath, f.configPath, f.rootfsPath, f.err
}

type storedImage struct {
	repository string
	reference  string
	bundlePath string
	configPath string
	rootfsPath string
}

type fakeImageIlmHandler struct {
	stored      storedImage
	bundlePath  string
	configPath  string
	rootfsPath  string
	info        ilm.ImageInfo
	removedRepo string
	removedRef  string
}

func (f *fakeImageIlmHandler) StoreImage(repository, reference, bundlePath, configPath, rootfsPath string) error {
	f.stored = storedImage{repository: repository, reference: reference, bundlePath: bundlePath, configPath: configPath, rootfsPath: rootfsPath}
	return nil
}
func (f *fakeImageIlmHandler) RemoveImage(repository string, reference string) error {
	f.removedRepo = repository
	f.removedRef = reference
	return nil
}
func (f *fakeImageIlmHandler) GetBundlePath(string, string) (string, error) {
	if f.bundlePath == "" {
		return "", errors.New("missing bundle")
	}
	return f.bundlePath, nil
}
func (f *fakeImageIlmHandler) GetConfigPath(string, string) (string, error) {
	return f.configPath, nil
}
func (f *fakeImageIlmHandler) GetRootfsPath(string, string) (string, error) {
	return f.rootfsPath, nil
}
func (f *fakeImageIlmHandler) GetImageInfo(string, string) (ilm.ImageInfo, error) {
	return f.info, nil
}
func (f *fakeImageIlmHandler) GetImageList() ([]ilm.ImageInfo, error) { return nil, nil }
func (f *fakeImageIlmHandler) IsImageExist(string, string) bool       { return false }

type fakeImageFilesystemHandler struct {
	readErr    error
	removedAll []string
}

func (f *fakeImageFilesystemHandler) MkdirAll(string, os.FileMode) error          { return nil }
func (f *fakeImageFilesystemHandler) ReadFile(string) ([]byte, error)             { return nil, f.readErr }
func (f *fakeImageFilesystemHandler) WriteFile(string, []byte, os.FileMode) error { return nil }
func (f *fakeImageFilesystemHandler) Open(string) (*os.File, error)               { return nil, nil }
func (f *fakeImageFilesystemHandler) OpenFile(string, int, os.FileMode) (*os.File, error) {
	return nil, nil
}
func (f *fakeImageFilesystemHandler) Copy(io.Writer, io.Reader) (int64, error) { return 0, nil }
func (f *fakeImageFilesystemHandler) Remove(string) error                      { return nil }
func (f *fakeImageFilesystemHandler) RemoveAll(path string) error {
	f.removedAll = append(f.removedAll, path)
	return nil
}
func (f *fakeImageFilesystemHandler) Rename(string, string) error { return nil }
func (f *fakeImageFilesystemHandler) IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
func (f *fakeImageFilesystemHandler) Flock(int, int) error            { return nil }
func (f *fakeImageFilesystemHandler) Chmod(string, os.FileMode) error { return nil }
