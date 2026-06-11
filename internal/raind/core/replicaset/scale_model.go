package replicaset

type ScaleRequestModel struct {
	Replicas int `json:"replicas"`
}

type ScaleResponseDataModel struct {
	ReplicaSetId string `json:"replicaSetId"`
	Replicas     int    `json:"replicas"`
}

type ScaleResponseModel struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    ScaleResponseDataModel `json:"data,omitempty"`
}

type ServiceReplicaSetScaleModel struct {
	Id       string
	Replicas int
}
