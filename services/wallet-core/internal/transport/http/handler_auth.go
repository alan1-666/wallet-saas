package httptransport

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const maxRequestBodyBytes int64 = 1024 * 1024

func (h *WithdrawHandler) ensureAccountActive(w http.ResponseWriter, r *http.Request, tenantID, accountID, action string) bool {
	if h.Registry == nil {
		return true
	}
	acc, err := h.Registry.GetAccount(r.Context(), tenantID, accountID)
	if err == sql.ErrNoRows {
		http.Error(w, "account not found", http.StatusNotFound)
		return false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	if strings.ToUpper(strings.TrimSpace(acc.Status)) != "ACTIVE" {
		http.Error(w, "account is not active for "+action, http.StatusForbidden)
		return false
	}
	return true
}

func requestIDFrom(r *http.Request, fallback string) string {
	v := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if v != "" {
		return v
	}
	if strings.TrimSpace(fallback) != "" {
		return generateRequestID(fallback)
	}
	return fallback
}

func generateRequestID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	p := strings.TrimSpace(prefix)
	if p == "" {
		p = "req"
	}
	return fmt.Sprintf("%s-%d-%s", p, time.Now().UnixNano(), hex.EncodeToString(b))
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return false
	}
	return true
}
