package ingress

import (
	"net/http"
	"time"

	apimodel "raind/internal/condenser/api/http/utils"
	"raind/internal/condenser/store/ism"
	"raind/internal/condenser/utils"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		ismHandler: ism.NewIsmManager(ism.NewIsmStore(utils.IsmStorePath)),
	}
}

type RequestHandler struct {
	ismHandler ism.IsmHandler
}

// GetIngressList godoc
// @Summary list ingresses
// @Description list ingress resources
// @Tags ingresses
// @Produce json
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/ingresses [get]
func (h *RequestHandler) GetIngressList(w http.ResponseWriter, r *http.Request) {
	namespaceFilter := r.URL.Query().Get("namespace")
	list, err := h.ismHandler.GetIngressList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
		return
	}

	res := make([]IngressSummary, 0, len(list))
	for _, in := range list {
		if namespaceFilter != "" && in.Namespace != namespaceFilter {
			continue
		}
		res = append(res, IngressSummary{
			IngressId: in.IngressId,
			Name:      in.Name,
			Namespace: in.Namespace,
			Rules:     in.Rules,
			TLSHosts:  in.TLSHosts,
			CreatedAt: in.CreatedAt.Format(time.RFC3339),
		})
	}

	apimodel.RespondSuccess(w, http.StatusOK, "ingress list", res)
}
