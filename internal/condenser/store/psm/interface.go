package psm

type PsmStoreHandler interface {
	SetPodState() error
}

type PsmHandler interface {
	StorePod(podId, templateId, name, namespace, uid, state, networkNS, ipcNS, utsNS, userNS string, labels, annotations map[string]string) error
	StorePodTemplate(templateId string, spec PodTemplateSpec) error
	GetPodTemplate(templateId string) (PodTemplateInfo, error)
	GetPodTemplateList() ([]PodTemplateInfo, error)
	AddContainerToPodTemplate(podId string, spec ContainerTemplateSpec) error
	RemovePodTemplate(templateId string) error
	StoreReplicaSet(replicaSetId string, spec ReplicaSetSpec) error
	GetReplicaSet(replicaSetId string) (ReplicaSetInfo, error)
	GetReplicaSetList() ([]ReplicaSetInfo, error)
	IsTemplateReferenced(templateId string) (bool, error)
	UpdateReplicaSetReplicas(replicaSetId string, replicas int) error
	RemoveReplicaSet(replicaSetId string) error
	RemovePod(podId string) error
	UpdatePod(podId string, state string) error
	UpdatePodStoppedByUser(podId string, stopped bool) error
	UpdatePodNamespaces(ownerPid int, podId, networkNS, ipcNS, utsNS, userNS string) error
	ResetPodNamespaces(podId string) error
	GetPodList() ([]PodInfo, error)
	GetPodById(podId string) (PodInfo, error)
	IsNameAlreadyUsed(name, namespace string) bool
	GetPodIdByName(name, namespace string) (string, error)
	ResolvePodId(str, namespace string) (string, error)
	IsPodExist(podId string) bool
	IsPodOwner(podId string) bool
	GetPodOwnerPid(podId string) (int, error)
}
