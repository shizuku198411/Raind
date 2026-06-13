package namespace

type NamespaceServiceHandler interface {
	Create(ServiceCreateModel) (NamespaceInfo, error)
	Remove(ServiceRemoveModel) (string, error)
	Get(name string) (NamespaceInfo, error)
	List() ([]NamespaceInfo, error)
	ResolveNetwork(name string) (string, error)
}
