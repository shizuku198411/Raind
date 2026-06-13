package namespace

import corenamespace "raind/internal/condenser/core/namespace"

type CreateNamespaceRequest struct {
	Name        string            `json:"name"`
	Network     string            `json:"network,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type NamespaceResponse = corenamespace.NamespaceInfo
