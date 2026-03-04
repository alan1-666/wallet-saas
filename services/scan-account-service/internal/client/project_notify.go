package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ProjectNotify struct {
	baseURL string
	token   string
	http    *http.Client
}

type ProjectDepositNotifyRequest struct {
	BizID   string `json:"biz_id,omitempty"`
	ChainID int64  `json:"chain_id"`
	TxHash  string `json:"tx_hash"`
	TxIndex int64  `json:"tx_index"`
	Address string `json:"address"`
	Asset   string `json:"asset"`
	Amount  string `json:"amount"`
}

type projectNotifyResponse struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

func NewProjectNotify(baseURL, token string, timeout time.Duration) *ProjectNotify {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &ProjectNotify{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   strings.TrimSpace(token),
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *ProjectNotify) DepositNotify(ctx context.Context, requestID string, req ProjectDepositNotifyRequest) error {
	raw, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/account/asset/deposit/notify", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", requestID)
	if p.token != "" {
		httpReq.Header.Set("x-deposit-token", p.token)
	}

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("project notify deposit failed: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out projectNotifyResponse
	if err := json.Unmarshal(body, &out); err == nil && out.Code != 0 && out.Code != 10000 {
		return fmt.Errorf("project notify business failed: code=%d message=%s", out.Code, strings.TrimSpace(out.Message))
	}
	return nil
}
