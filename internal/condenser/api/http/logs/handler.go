package logs

import (
	"net/http"
	apimodel "raind/internal/condenser/api/http/utils"
	"raind/internal/condenser/core/logs"
	"strconv"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		serviceHandler: logs.NewLogService(),
	}
}

type RequestHandler struct {
	serviceHandler logs.LogServiceHandler
}

func (h *RequestHandler) GetNetflowLog(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if s := query.Get("tail_lines"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			apimodel.RespondFail(w, http.StatusBadRequest, "invalid  tail_lines", nil)
			return
		}
		data, err := h.serviceHandler.GetNetflowLogWithTailLines(n)
		if err != nil {
			apimodel.RespondFail(w, http.StatusInternalServerError, "tail failed: "+err.Error(), nil)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
		return
	}
}
