package vsm

import "time"

const (
	PVCPhaseBound    = "Bound"
	PVCPhaseReleased = "Released"

	PVCReclaimRetain = "Retain"
	PVCReclaimDelete = "Delete"
)

type PersistentVolumeClaimInfo struct {
	PVCId            string    `json:"pvcId"`
	Name             string    `json:"name"`
	Namespace        string    `json:"namespace"`
	AccessModes      []string  `json:"accessModes"`
	RequestedStorage string    `json:"requestedStorage"`
	RequestedBytes   uint64    `json:"requestedBytes"`
	StorageClassName string    `json:"storageClassName,omitempty"`
	VolumeMode       string    `json:"volumeMode,omitempty"`
	HostPath         string    `json:"hostPath"`
	DataPath         string    `json:"dataPath"`
	ReclaimPolicy    string    `json:"reclaimPolicy"`
	Phase            string    `json:"phase"`
	CreatedAt        time.Time `json:"createdAt"`
}

type VolumeState struct {
	Version string                               `json:"version"`
	PVCs    map[string]PersistentVolumeClaimInfo `json:"persistentVolumeClaims"`
}
