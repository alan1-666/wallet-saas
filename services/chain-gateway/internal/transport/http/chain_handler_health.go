package httptransport

import (
	"net/http"
	"time"
)

type HealthResponse struct {
	Status     string `json:"status"`
	ServerTime string `json:"server_time"`
	Endpoints  any    `json:"endpoints,omitempty"`
}

func (h *ChainHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := HealthResponse{
		Status:     "ok",
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	}
	if h.Endpoints != nil {
		resp.Endpoints = h.Endpoints.Snapshot()
	}
	writeJSON(w, resp)
}
