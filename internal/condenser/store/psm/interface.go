package psm

import "time"

type PsmStoreHandler interface {
	SetPodState() error
}

type PsmHandler interface {
	StorePod(req StorePodRequest) error
	StorePodTemplate(templateId string, spec PodTemplateSpec) error
	GetPodTemplate(templateId string) (PodTemplateInfo, error)
	GetPodTemplateList() ([]PodTemplateInfo, error)
	AddContainerToPodTemplate(podId string, spec ContainerTemplateSpec) error
	RemovePodTemplate(templateId string) error
	StoreReplicaSet(replicaSetId string, spec ReplicaSetSpec) error
	GetReplicaSet(replicaSetId string) (ReplicaSetInfo, error)
	GetReplicaSetList() ([]ReplicaSetInfo, error)
	IsTemplateReferenced(templateId string) (bool, error)
	UpdateReplicaSetSpec(replicaSetId string, spec ReplicaSetSpec) error
	UpdateReplicaSetReplicas(replicaSetId string, replicas int) error
	UpdateReplicaSetReconcileStatus(replicaSetId string, attempt int, lastError string, nextReconcileAt time.Time) error
	ClearReplicaSetReconcileStatus(replicaSetId string) error
	RemoveReplicaSet(replicaSetId string) error
	StoreDeployment(deploymentId string, spec DeploymentSpec) error
	GetDeployment(deploymentId string) (DeploymentInfo, error)
	GetDeploymentList() ([]DeploymentInfo, error)
	UpdateDeploymentSpec(deploymentId string, spec DeploymentSpec) error
	UpdateDeploymentReplicas(deploymentId string, replicas int) error
	UpdateDeploymentReplicaSet(deploymentId, replicaSetId string) error
	RemoveDeployment(deploymentId string) error
	RemovePod(podId string) error
	UpdatePod(podId string, state string) error
	UpdatePodOwner(podId, ownerKind, ownerId string) error
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
