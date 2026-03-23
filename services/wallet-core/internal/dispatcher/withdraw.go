package dispatcher

import (
	"context"
	"log"
	"sync"
	"time"

	"wallet-saas-v2/services/wallet-core/internal/orchestrator"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type WithdrawDispatcher struct {
	Ledger                ports.LedgerPort
	Orch                  *orchestrator.WithdrawOrchestrator
	Interval              time.Duration
	Batch                 int
	AccelerateBatch       int
	Parallelism           int
	MaxAttempts           int
	BaseBackoff           time.Duration
	MaxBackoff            time.Duration
	AccelerateAfter       time.Duration
	AccelerateMaxAttempts int
	AccelerateGasBumpBps  int
}

func (d *WithdrawDispatcher) Run(ctx context.Context) {
	interval := d.Interval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	d.dispatchOnce(ctx)
	d.accelerateOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.dispatchOnce(ctx)
			d.accelerateOnce(ctx)
		}
	}
}

func (d *WithdrawDispatcher) dispatchOnce(ctx context.Context) {
	if d == nil || d.Ledger == nil || d.Orch == nil {
		return
	}
	batch := d.Batch
	if batch <= 0 {
		batch = 8
	}
	jobs, err := d.Ledger.ClaimQueuedWithdraws(ctx, batch)
	if err != nil {
		log.Printf("withdraw dispatcher claim failed err=%v", err)
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
			d.handleJob(ctx, job)
		}()
	}
	wg.Wait()
}

func (d *WithdrawDispatcher) shouldRetry(attempt int) bool {
	maxAttempts := d.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return attempt < maxAttempts
}

func (d *WithdrawDispatcher) retryDelay(attempt int) time.Duration {
	base := d.BaseBackoff
	if base <= 0 {
		base = time.Second
	}
	maxDelay := d.MaxBackoff
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}
	if attempt < 1 {
		attempt = 1
	}
	delay := base * time.Duration(1<<minInt(attempt-1, 6))
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func (d *WithdrawDispatcher) parallelism(jobCount int) int {
	parallelism := d.Parallelism
	if parallelism <= 0 {
		parallelism = 4
	}
	if jobCount > 0 && parallelism > jobCount {
		return jobCount
	}
	return parallelism
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (d *WithdrawDispatcher) handleJob(ctx context.Context, job ports.WithdrawJob) {
	req := orchestrator.WithdrawRequest{
		TenantID:      job.TenantID,
		AccountID:     job.AccountID,
		OrderID:       job.OrderID,
		RequiredConfs: job.RequiredConfs,
		Signers:       job.Signers,
		SignType:      job.SignType,
		Tx:            job.Tx,
	}
	broadcast, err := d.Orch.ExecuteQueuedWithUnsigned(ctx, req)
	if err != nil {
		reason := err.Error()
		if d.shouldRetry(job.AttemptCount) {
			delay := d.retryDelay(job.AttemptCount)
			if markErr := d.Ledger.RescheduleQueuedWithdraw(ctx, job.TenantID, job.OrderID, reason, delay); markErr != nil {
				log.Printf("withdraw dispatcher reschedule failed tenant=%s order=%s err=%v", job.TenantID, job.OrderID, markErr)
			}
			log.Printf("withdraw dispatcher requeued tenant=%s order=%s attempt=%d next_in=%s err=%v", job.TenantID, job.OrderID, job.AttemptCount, delay, err)
			return
		}
		if releaseErr := d.Orch.ReleaseQueued(ctx, req, reason); releaseErr != nil {
			reason = reason + "; release failed: " + releaseErr.Error()
		}
		if markErr := d.Ledger.MarkQueuedWithdrawFailed(ctx, job.TenantID, job.OrderID, reason); markErr != nil {
			log.Printf("withdraw dispatcher mark failed tenant=%s order=%s err=%v", job.TenantID, job.OrderID, markErr)
		}
		log.Printf("withdraw dispatcher execute failed tenant=%s order=%s err=%v", job.TenantID, job.OrderID, err)
		return
	}
	if err := d.Ledger.MarkQueuedWithdrawDone(ctx, job.TenantID, job.OrderID, broadcast.TxHash, broadcast.UnsignedTx); err != nil {
		log.Printf("withdraw dispatcher mark done failed tenant=%s order=%s tx=%s err=%v", job.TenantID, job.OrderID, broadcast.TxHash, err)
		return
	}
	log.Printf("withdraw dispatcher broadcasted tenant=%s order=%s tx=%s", job.TenantID, job.OrderID, broadcast.TxHash)
}
