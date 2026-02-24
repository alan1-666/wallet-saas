package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type WalletCore struct {
	baseURL string
	token   string
	http    *http.Client
}

type DepositNotifyRequest struct {
	TenantID      string `json:"tenant_id"`
	AccountID     string `json:"account_id"`
	OrderID       string `json:"order_id"`
	Chain         string `json:"chain"`
	Coin          string `json:"coin"`
	Amount        string `json:"amount"`
	TxHash        string `json:"tx_hash"`
	FromAddress   string `json:"from_address"`
	ToAddress     string `json:"to_address"`
	Confirmations int64  `json:"confirmations"`
	RequiredConfs int64  `json:"required_confirmations"`
	Status        string `json:"status"`
}

type SweepRunRequest struct {
	TenantID          string `json:"tenant_id"`
	SweepOrderID      string `json:"sweep_order_id"`
	FromAccountID     string `json:"from_account_id"`
	TreasuryAccountID string `json:"treasury_account_id"`
	Asset             string `json:"asset"`
	Amount            string `json:"amount"`
}

func NewWalletCore(baseURL, token string) *WalletCore {
	return &WalletCore{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (w *WalletCore) DepositNotify(ctx context.Context, requestID string, req DepositNotifyRequest) error {
	return w.postJSON(ctx, "/v1/deposit/notify", requestID, req)
}

func (w *WalletCore) SweepRun(ctx context.Context, requestID string, req SweepRunRequest) error {
	return w.postJSON(ctx, "/v1/sweep/run", requestID, req)
}

func (w *WalletCore) postJSON(ctx context.Context, path, requestID string, body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, w.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", requestID)
	if w.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+w.token)
	}

	resp, err := w.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("wallet-core %s failed: %s", path, resp.Status)
	}
	return nil
}
