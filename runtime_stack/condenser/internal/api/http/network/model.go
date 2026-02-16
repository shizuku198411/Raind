package network

type CreateBridgeRequest struct {
	Bridge string `json:"bridge"`
}

type DeleteBridgeRequest struct {
	Bridge string `json:"bridge"`
}
