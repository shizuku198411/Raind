package pod

type PodServiceHandler interface {
	Create(createParameter ServiceCreateModel) (string, error)
	RecreateFromTemplate(templateId string) (string, error)
	CreateFromTemplate(templateId string, nameOverride string) (string, error)
	Start(podId string) (string, error)
	Stop(podId string) (string, error)
	Remove(podId string) (string, error)
	GetPodList() ([]PodState, error)
	GetPodById(podId string) (PodState, error)
}
