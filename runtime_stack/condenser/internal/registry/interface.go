package registry

type RegistryHandler interface {
	PullImage(pullParameter RegistryPullModel) (repository, reference, bundlePath, configPath, rootfsPath string, err error)
}
