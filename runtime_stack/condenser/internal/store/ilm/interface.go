package ilm

type IlmStoreHandler interface {
	SetConfig() error
}

type IlmHandler interface {
	StoreImage(repository, reference, bundlePath, configPath, rootfsPath string) error
	RemoveImage(repository string, reference string) error
	GetBundlePath(repository string, reference string) (string, error)
	GetConfigPath(repository string, reference string) (string, error)
	GetRootfsPath(repository string, reference string) (string, error)
	GetImageInfo(repository string, reference string) (ImageInfo, error)
	GetImageList() ([]ImageInfo, error)
	IsImageExist(imageRepo, imageRef string) bool
}
