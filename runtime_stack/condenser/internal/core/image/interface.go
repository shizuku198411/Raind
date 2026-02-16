package image

type ImageServiceHandler interface {
	Pull(pullParameter ServicePullModel) error
	Remove(removeParameter ServiceRemoveModel) error
	Build(buildParameter ServiceBuildModel) (string, error)
	GetImageConfig(filepath string) (ImageConfigFile, error)
	GetImageList() ([]ImageInfo, error)
	GetImageStatus(imageStr string) (ImageStatusInfo, error)
	GetImageFsInfo(imageStr string) (ImageFsInfo, error)
}
