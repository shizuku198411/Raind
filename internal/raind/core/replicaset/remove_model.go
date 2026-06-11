package replicaset

type RemoveResponseDataModel struct {
	ReplicaSetId string `json:"replicaSetId,omitempty"`
}

type RemoveResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    RemoveResponseDataModel `json:"data,omitempty"`
}

type ServiceReplicaSetRemoveModel struct {
	Id string
}
