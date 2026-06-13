package nsm

type NsmStoreHandler interface {
	SetNamespaceState() error
}

type NsmHandler interface {
	EnsureDefaultNamespace() error
	StoreNamespace(info NamespaceInfo) error
	GetNamespace(name string) (NamespaceInfo, error)
	GetNamespaceList() ([]NamespaceInfo, error)
	RemoveNamespace(name string) error
	IsNamespaceExist(name string) bool
}
