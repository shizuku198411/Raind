package resource

type DeletedPodModel struct {
	PodId     string `json:"podId,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type DeletedReplicaSetModel struct {
	ReplicaSetId string `json:"replicaSetId,omitempty"`
	Name         string `json:"name,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
}

type DeletedServiceModel struct {
	ServiceId string `json:"serviceId,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type DeletedDeploymentModel struct {
	DeploymentId string `json:"deploymentId,omitempty"`
	Name         string `json:"name,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
}

type DeletedNamespaceModel struct {
	Name string `json:"name,omitempty"`
}

type DeletedIngressModel struct {
	IngressId string `json:"ingressId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type DeleteResponseModel struct {
	Pods        []DeletedPodModel        `json:"pods,omitempty"`
	ReplicaSets []DeletedReplicaSetModel `json:"replicasets,omitempty"`
	Deployments []DeletedDeploymentModel `json:"deployments,omitempty"`
	Services    []DeletedServiceModel    `json:"services,omitempty"`
	Ingresses   []DeletedIngressModel    `json:"ingresses,omitempty"`
	Namespaces  []DeletedNamespaceModel  `json:"namespaces,omitempty"`
}

type DeleteApiResponseModel struct {
	Status  string              `json:"status"`
	Message string              `json:"message"`
	Data    DeleteResponseModel `json:"data,omitempty"`
}

type ServiceResourceDeleteModel struct {
	FilePath string
}
