package pvc

import "time"

type PVCInfo struct {
	PVCId            string    `json:"pvcId"`
	Name             string    `json:"name"`
	Namespace        string    `json:"namespace"`
	Phase            string    `json:"phase"`
	AccessModes      []string  `json:"accessModes"`
	RequestedStorage string    `json:"requestedStorage"`
	RequestedBytes   uint64    `json:"requestedBytes"`
	ReclaimPolicy    string    `json:"reclaimPolicy"`
	DataPath         string    `json:"dataPath"`
	CreatedAt        time.Time `json:"createdAt"`
}

type ListResponseModel struct {
	Status  string    `json:"status"`
	Message string    `json:"message"`
	Data    []PVCInfo `json:"data,omitempty"`
}

type DetailResponseModel struct {
	Status  string  `json:"status"`
	Message string  `json:"message"`
	Data    PVCInfo `json:"data,omitempty"`
}

type RemoveResponseModel struct {
	Status  string  `json:"status"`
	Message string  `json:"message"`
	Data    PVCInfo `json:"data,omitempty"`
}
