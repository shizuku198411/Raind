package resource

type ApplyResult struct {
	Pods                   []ApplyPodResult           `json:"pods"`
	ReplicaSets            []ApplyReplicaSetResult    `json:"replicasets"`
	Deployments            []ApplyDeploymentResult    `json:"deployments"`
	Services               []ApplyServiceResult       `json:"services"`
	Ingresses              []ApplyIngressResult       `json:"ingresses"`
	Namespaces             []ApplyNamespaceResult     `json:"namespaces"`
	ConfigMaps             []ApplyConfigMapResult     `json:"configMaps"`
	Secrets                []ApplySecretResult        `json:"secrets"`
	NetworkPolicies        []ApplyNetworkPolicyResult `json:"networkPolicies"`
	PersistentVolumeClaims []ApplyPVCResult           `json:"persistentVolumeClaims"`
	Warnings               []Warning                  `json:"warnings,omitempty"`
}

type ApplyPodResult struct {
	PodId        string   `json:"podId"`
	ReplicaSetId string   `json:"replicaSetId,omitempty"`
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	ContainerIds []string `json:"containerIds"`
	Action       string   `json:"action,omitempty"`
}

type ApplyServiceResult struct {
	ServiceId string `json:"serviceId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Action    string `json:"action,omitempty"`
}

type ApplyIngressResult struct {
	IngressId string   `json:"ingressId"`
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	TLSHosts  []string `json:"tlsHosts,omitempty"`
	Action    string   `json:"action,omitempty"`
}

type ApplyNamespaceResult struct {
	Name    string `json:"name"`
	Network string `json:"network"`
	Action  string `json:"action,omitempty"`
}

type ApplyConfigMapResult struct {
	ConfigMapId string `json:"configMapId"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Action      string `json:"action,omitempty"`
}

type ApplySecretResult struct {
	SecretId  string `json:"secretId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Action    string `json:"action,omitempty"`
}

type ApplyNetworkPolicyResult struct {
	NetworkPolicyId string `json:"networkPolicyId"`
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	GeneratedRules  int    `json:"generatedRules"`
	Action          string `json:"action,omitempty"`
}

type ApplyPVCResult struct {
	PVCId            string `json:"pvcId"`
	Name             string `json:"name"`
	Namespace        string `json:"namespace"`
	RequestedStorage string `json:"requestedStorage"`
	RequestedBytes   uint64 `json:"requestedBytes"`
	ReclaimPolicy    string `json:"reclaimPolicy"`
	Action           string `json:"action,omitempty"`
}

type ApplyReplicaSetResult struct {
	ReplicaSetId string `json:"replicaSetId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Action       string `json:"action,omitempty"`
}

type ApplyDeploymentResult struct {
	DeploymentId string `json:"deploymentId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Replicas     int    `json:"replicas"`
	Action       string `json:"action,omitempty"`
}

type DeleteResult struct {
	Pods                   []DeletePodResult           `json:"pods"`
	ReplicaSets            []DeleteReplicaSetResult    `json:"replicasets"`
	Deployments            []DeleteDeploymentResult    `json:"deployments"`
	Services               []DeleteServiceResult       `json:"services"`
	Ingresses              []DeleteIngressResult       `json:"ingresses"`
	Namespaces             []DeleteNamespaceResult     `json:"namespaces"`
	ConfigMaps             []DeleteConfigMapResult     `json:"configMaps"`
	Secrets                []DeleteSecretResult        `json:"secrets"`
	NetworkPolicies        []DeleteNetworkPolicyResult `json:"networkPolicies"`
	PersistentVolumeClaims []DeletePVCResult           `json:"persistentVolumeClaims"`
	Warnings               []Warning                   `json:"warnings,omitempty"`
}

type DeletePodResult struct {
	PodId     string `json:"podId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DeleteReplicaSetResult struct {
	ReplicaSetId string `json:"replicaSetId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
}

type DeleteServiceResult struct {
	ServiceId string `json:"serviceId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DeleteIngressResult struct {
	IngressId string `json:"ingressId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DeleteDeploymentResult struct {
	DeploymentId string `json:"deploymentId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
}

type DeleteNamespaceResult struct {
	Name string `json:"name"`
}

type DeleteConfigMapResult struct {
	ConfigMapId string `json:"configMapId"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
}

type DeleteSecretResult struct {
	SecretId  string `json:"secretId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DeleteNetworkPolicyResult struct {
	NetworkPolicyId string `json:"networkPolicyId"`
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
}

type DeletePVCResult struct {
	PVCId         string `json:"pvcId"`
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	ReclaimPolicy string `json:"reclaimPolicy"`
}

type Warning struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Field     string `json:"field,omitempty"`
	Message   string `json:"message"`
}
