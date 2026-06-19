package pod

type ServicePodApplyModel struct {
	FilePath string
}

type ApplyResponseDataModel struct {
	Pods        []PodInfo        `json:"pods"`
	ReplicaSets []ReplicaSetInfo `json:"replicasets"`
	Deployments []DeploymentInfo `json:"deployments"`
	Services    []ServiceInfo    `json:"services"`
	Ingresses   []IngressInfo    `json:"ingresses"`
	Namespaces  []NamespaceInfo  `json:"namespaces"`
	ConfigMaps  []ConfigMapInfo  `json:"configMaps"`
	Secrets     []SecretInfo     `json:"secrets"`
	Warnings    []WarningInfo    `json:"warnings,omitempty"`
}

type PodInfo struct {
	PodId        string   `json:"podId"`
	ReplicaSetId string   `json:"replicaSetId,omitempty"`
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	ContainerIds []string `json:"containerIds"`
}

type ReplicaSetInfo struct {
	ReplicaSetId string `json:"replicaSetId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
}

type DeploymentInfo struct {
	DeploymentId string `json:"deploymentId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Replicas     int    `json:"replicas"`
}

type ServiceInfo struct {
	ServiceId string `json:"serviceId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type IngressInfo struct {
	IngressId string   `json:"ingressId"`
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	TLSHosts  []string `json:"tlsHosts,omitempty"`
}

type NamespaceInfo struct {
	Name    string `json:"name"`
	Network string `json:"network"`
}

type ConfigMapInfo struct {
	ConfigMapId string `json:"configMapId"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
}

type SecretInfo struct {
	SecretId  string `json:"secretId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type WarningInfo struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Field     string `json:"field,omitempty"`
	Message   string `json:"message"`
}

type ApplyResponseModel struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    ApplyResponseDataModel `json:"data,omitempty"`
}
