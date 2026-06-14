package ism

type IsmHandler interface {
	StoreIngress(ingressId string, spec IngressInfo) error
	GetIngressList() ([]IngressInfo, error)
	GetIngressById(ingressId string) (IngressInfo, error)
	RemoveIngress(ingressId string) error
	IsNameAlreadyUsed(name, namespace string) bool
}
