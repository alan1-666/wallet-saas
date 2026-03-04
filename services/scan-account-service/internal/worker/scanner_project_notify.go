package worker

import (
	"context"
	"fmt"
	"strings"

	"wallet-saas-v2/services/scan-account-service/internal/client"
)

func (s *Scanner) notifyProjectDeposit(ctx context.Context, requestID string, payload DepositOutboxPayload) error {
	if s.ProjectNotify == nil {
		return nil
	}
	if strings.ToUpper(strings.TrimSpace(payload.Status)) != "CONFIRMED" {
		return nil
	}
	chainID, ok := s.resolveProjectChainID(payload.Chain, payload.Network)
	if !ok {
		return fmt.Errorf("project notify chain id not configured chain=%s network=%s", payload.Chain, payload.Network)
	}

	address := strings.TrimSpace(payload.WatchAddress)
	if address == "" {
		address = strings.TrimSpace(payload.ToAddress)
	}
	if address == "" {
		return fmt.Errorf("project notify address empty tenant=%s order=%s tx=%s", payload.TenantID, payload.OrderID, payload.TxHash)
	}

	bizID := fmt.Sprintf("walletsaas:%s:%s:%s:%d",
		strings.ToLower(strings.TrimSpace(payload.Chain)),
		strings.ToLower(strings.TrimSpace(payload.Network)),
		strings.TrimSpace(payload.TxHash),
		payload.EventIndex,
	)
	return s.ProjectNotify.DepositNotify(ctx, requestID, client.ProjectDepositNotifyRequest{
		BizID:   bizID,
		ChainID: chainID,
		TxHash:  strings.TrimSpace(payload.TxHash),
		TxIndex: payload.EventIndex,
		Address: address,
		Asset:   strings.ToUpper(strings.TrimSpace(payload.Coin)),
		Amount:  strings.TrimSpace(payload.Amount),
	})
}

func (s *Scanner) resolveProjectChainID(chain, network string) (int64, bool) {
	key := strings.ToLower(strings.TrimSpace(chain)) + ":" + strings.ToLower(strings.TrimSpace(network))
	if s.ProjectChainIDMap != nil {
		if id, ok := s.ProjectChainIDMap[key]; ok && id > 0 {
			return id, true
		}
	}
	if s.ProjectDefaultChainID > 0 {
		return s.ProjectDefaultChainID, true
	}
	return 0, false
}
