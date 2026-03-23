package dispatcher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"wallet-saas-v2/services/wallet-core/internal/orchestrator"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type evmDynamicFeePayload struct {
	ChainID              string `json:"chainId"`
	NonceMode            string `json:"nonceMode,omitempty"`
	FromAddress          string `json:"fromAddress,omitempty"`
	Nonce                uint64 `json:"nonce"`
	MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         string `json:"maxFeePerGas"`
	GasLimit             uint64 `json:"gasLimit"`
	ToAddress            string `json:"toAddress"`
	Amount               string `json:"amount"`
	ContractAddress      string `json:"contractAddress,omitempty"`
}

func (d *WithdrawDispatcher) accelerateOnce(ctx context.Context) {
	if d == nil || d.Ledger == nil || d.Orch == nil {
		return
	}
	batch := d.AccelerateBatch
	if batch <= 0 {
		batch = 8
	}
	jobs, err := d.Ledger.ClaimStaleBroadcastedWithdraws(ctx, batch, d.accelerateAfter(), d.accelerateMaxAttempts())
	if err != nil {
		log.Printf("withdraw accelerator claim failed err=%v", err)
		return
	}
	if len(jobs) == 0 {
		return
	}
	parallelism := d.parallelism(len(jobs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallelism)
	for _, job := range jobs {
		job := job
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			d.handleAcceleration(ctx, job)
		}()
	}
	wg.Wait()
}

func (d *WithdrawDispatcher) handleAcceleration(ctx context.Context, job ports.WithdrawJob) {
	replacementPayload, err := bumpEVMReplacementPayload(job.Tx.Base64Tx, d.accelerateGasBumpBps())
	if err != nil {
		_ = d.Ledger.ResetBroadcastedWithdraw(ctx, job.TenantID, job.OrderID, err.Error())
		log.Printf("withdraw accelerator payload failed tenant=%s order=%s tx=%s err=%v", job.TenantID, job.OrderID, job.TxHash, err)
		return
	}
	req := orchestrator.WithdrawRequest{
		TenantID:      job.TenantID,
		AccountID:     job.AccountID,
		OrderID:       job.OrderID,
		RequiredConfs: job.RequiredConfs,
		Signers:       job.Signers,
		SignType:      job.SignType,
		Tx:            job.Tx,
	}
	req.Tx.Base64Tx = replacementPayload
	broadcast, err := d.Orch.SpeedUpBroadcasted(ctx, req)
	if err != nil {
		_ = d.Ledger.ResetBroadcastedWithdraw(ctx, job.TenantID, job.OrderID, err.Error())
		log.Printf("withdraw accelerator broadcast failed tenant=%s order=%s tx=%s err=%v", job.TenantID, job.OrderID, job.TxHash, err)
		return
	}
	if err := d.Ledger.ReplaceBroadcastedWithdraw(ctx, job.TenantID, job.OrderID, job.TxHash, broadcast.TxHash, broadcast.UnsignedTx); err != nil {
		log.Printf("withdraw accelerator replace failed tenant=%s order=%s old_tx=%s new_tx=%s err=%v", job.TenantID, job.OrderID, job.TxHash, broadcast.TxHash, err)
		return
	}
	log.Printf("withdraw accelerator replaced tenant=%s order=%s old_tx=%s new_tx=%s", job.TenantID, job.OrderID, job.TxHash, broadcast.TxHash)
}

func (d *WithdrawDispatcher) accelerateAfter() time.Duration {
	if d.AccelerateAfter <= 0 {
		return time.Minute
	}
	return d.AccelerateAfter
}

func (d *WithdrawDispatcher) accelerateMaxAttempts() int {
	if d.AccelerateMaxAttempts <= 0 {
		return 3
	}
	return d.AccelerateMaxAttempts
}

func (d *WithdrawDispatcher) accelerateGasBumpBps() int {
	if d.AccelerateGasBumpBps <= 0 {
		return 2000
	}
	return d.AccelerateGasBumpBps
}

func bumpEVMReplacementPayload(base64Tx string, bumpBps int) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(base64Tx)
	if err != nil {
		return "", fmt.Errorf("invalid replacement payload: %w", err)
	}
	var payload evmDynamicFeePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("invalid replacement payload json: %w", err)
	}
	if bumpBps <= 0 {
		bumpBps = 2000
	}
	priority, err := bumpDecimalString(payload.MaxPriorityFeePerGas, bumpBps)
	if err != nil {
		return "", fmt.Errorf("bump priority fee: %w", err)
	}
	maxFee, err := bumpDecimalString(payload.MaxFeePerGas, bumpBps)
	if err != nil {
		return "", fmt.Errorf("bump max fee: %w", err)
	}
	if compareDecimalString(maxFee, priority) < 0 {
		maxFee = priority
	}
	payload.NonceMode = "fixed"
	payload.MaxPriorityFeePerGas = priority
	payload.MaxFeePerGas = maxFee
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encoded), nil
}

func bumpDecimalString(v string, bumpBps int) (string, error) {
	n, ok := new(big.Int).SetString(v, 10)
	if !ok {
		return "", fmt.Errorf("invalid integer amount: %s", v)
	}
	if n.Sign() < 0 {
		return "", fmt.Errorf("invalid negative integer amount: %s", v)
	}
	if n.Sign() == 0 {
		return "1", nil
	}
	multiplier := big.NewInt(int64(10000 + bumpBps))
	out := new(big.Int).Mul(n, multiplier)
	out.Div(out, big.NewInt(10000))
	if out.Cmp(n) <= 0 {
		out = new(big.Int).Add(n, big.NewInt(1))
	}
	return out.String(), nil
}

func compareDecimalString(a, b string) int {
	aa, ok := new(big.Int).SetString(a, 10)
	if !ok {
		return 0
	}
	bb, ok := new(big.Int).SetString(b, 10)
	if !ok {
		return 0
	}
	return aa.Cmp(bb)
}
