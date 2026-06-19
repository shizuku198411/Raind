package pvc

type PVCSummary struct {
	PVCId            string   `json:"pvcId"`
	Name             string   `json:"name"`
	Namespace        string   `json:"namespace"`
	Phase            string   `json:"phase"`
	AccessModes      []string `json:"accessModes"`
	RequestedStorage string   `json:"requestedStorage"`
	RequestedBytes   uint64   `json:"requestedBytes"`
	ReclaimPolicy    string   `json:"reclaimPolicy"`
	DataPath         string   `json:"dataPath"`
	CreatedAt        string   `json:"createdAt"`
}
