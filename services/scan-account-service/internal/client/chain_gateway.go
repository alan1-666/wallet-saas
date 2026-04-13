package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	pb "wallet-saas-v2/services/scan-account-service/internal/pb/chaingateway"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type ChainGateway struct {
	conn    *grpc.ClientConn
	client  pb.ChainGatewayServiceClient
	timeout time.Duration
	opts    ChainGatewayOptions
	mu      sync.Mutex
	gates   map[string]*chainRequestGate
}

type ChainGatewayOptions struct {
	DefaultQPS         float64
	DefaultConcurrency int
	QPSByChain         map[string]float64
	ConcurrencyByChain map[string]int
	RetryMaxAttempts   int
	RetryBaseDelay     time.Duration
	RetryMaxDelay      time.Duration
}

type chainRequestGate struct {
	sem      chan struct{}
	interval time.Duration

	mu   sync.Mutex
	next time.Time
}

type IncomingTransfer struct {
	TxHash          string
	FromAddress     string
	ToAddress       string
	Amount          string
	Confirmations   int64
	Index           int64
	Status          string
	ContractAddress string
}

type BlockMeta struct {
	Number     int64
	Hash       string
	ParentHash string
}

type TxFinality struct {
	TxHash        string
	Confirmations int64
	Status        string
	Found         bool
}

func NewChainGateway(addr string, timeout time.Duration, optList ...ChainGatewayOptions) (*ChainGateway, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	var grpcCreds grpc.DialOption
	if os.Getenv("GRPC_TLS_ENABLED") == "true" {
		grpcCreds = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}))
	} else {
		grpcCreds = grpc.WithTransportCredentials(insecure.NewCredentials())
	}
	conn, err := grpc.NewClient(addr, grpcCreds)
	if err != nil {
		return nil, err
	}
	opts := defaultChainGatewayOptions()
	if len(optList) > 0 {
		opts = mergeChainGatewayOptions(opts, optList[0])
	}
	return &ChainGateway{
		conn:    conn,
		client:  pb.NewChainGatewayServiceClient(conn),
		timeout: timeout,
		opts:    opts,
		gates:   make(map[string]*chainRequestGate),
	}, nil
}

func (c *ChainGateway) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

type IncomingTransferResult struct {
	Items      []IncomingTransfer
	NextCursor string
	Blocks     []BlockMeta
}

func (c *ChainGateway) ListIncomingTransfers(ctx context.Context, model, chain, coin, network, address, cursor string, pageSize int, contractAddress ...string) (IncomingTransferResult, error) {
	ca := ""
	if len(contractAddress) > 0 {
		ca = strings.TrimSpace(contractAddress[0])
	}
	out, err := withChainControl(c, ctx, chain, func(callCtx context.Context) (IncomingTransferResult, error) {
		callCtx, cancel := context.WithTimeout(callCtx, c.timeout)
		defer cancel()
		network = strings.TrimSpace(network)
		if network == "" {
			return IncomingTransferResult{}, fmt.Errorf("network is required")
		}
		resp, err := c.client.ListIncomingTransfers(callCtx, &pb.ListIncomingTransfersRequest{
			Model:           model,
			Chain:           chain,
			Coin:            coin,
			Network:         network,
			Address:         address,
			Page:            1,
			PageSize:        uint32(pageSize),
			Cursor:          cursor,
			ContractAddress: ca,
		})
		if err != nil {
			return IncomingTransferResult{}, err
		}
		items := make([]IncomingTransfer, 0, len(resp.GetItems()))
		for _, item := range resp.Items {
			items = append(items, IncomingTransfer{
				TxHash:          item.GetTxHash(),
				FromAddress:     item.GetFromAddress(),
				ToAddress:       item.GetToAddress(),
				Index:           item.GetIndex(),
				Amount:          item.GetAmount(),
				Confirmations:   item.GetConfirmations(),
				Status:          item.GetStatus(),
				ContractAddress: item.GetContractAddress(),
			})
		}
		blocks := make([]BlockMeta, 0, len(resp.GetBlocks()))
		for _, b := range resp.GetBlocks() {
			blocks = append(blocks, BlockMeta{
				Number:     b.GetNumber(),
				Hash:       b.GetHash(),
				ParentHash: b.GetParentHash(),
			})
		}
		return IncomingTransferResult{Items: items, NextCursor: resp.GetNextCursor(), Blocks: blocks}, nil
	})
	if err != nil {
		return IncomingTransferResult{}, err
	}
	return out, nil
}

func (c *ChainGateway) TxFinality(ctx context.Context, chain, coin, network, txHash string) (TxFinality, error) {
	return withChainControl(c, ctx, chain, func(callCtx context.Context) (TxFinality, error) {
		callCtx, cancel := context.WithTimeout(callCtx, c.timeout)
		defer cancel()
		network = strings.TrimSpace(network)
		if network == "" {
			return TxFinality{}, fmt.Errorf("network is required")
		}
		out, err := c.client.GetTxFinality(callCtx, &pb.TxFinalityRequest{
			Chain:   chain,
			Coin:    coin,
			Network: network,
			TxHash:  txHash,
		})
		if err != nil {
			return TxFinality{}, err
		}
		return TxFinality{
			TxHash:        out.GetTxHash(),
			Confirmations: out.GetConfirmations(),
			Status:        strings.ToUpper(strings.TrimSpace(out.GetStatus())),
			Found:         out.GetFound(),
		}, nil
	})
}

func (c *ChainGateway) GetBalance(ctx context.Context, chain, coin, network, address string) (string, error) {
	type result struct {
		balance string
	}
	out, err := withChainControl(c, ctx, chain, func(callCtx context.Context) (result, error) {
		callCtx, cancel := context.WithTimeout(callCtx, c.timeout)
		defer cancel()
		network = strings.TrimSpace(network)
		if network == "" {
			return result{}, fmt.Errorf("network is required")
		}
		resp, err := c.client.GetBalance(callCtx, &pb.BalanceRequest{
			Chain:   chain,
			Coin:    coin,
			Network: network,
			Address: address,
		})
		if err != nil {
			return result{}, err
		}
		return result{balance: strings.TrimSpace(resp.GetBalance())}, nil
	})
	if err != nil {
		return "", err
	}
	return out.balance, nil
}

func defaultChainGatewayOptions() ChainGatewayOptions {
	return ChainGatewayOptions{
		DefaultQPS:         2,
		DefaultConcurrency: 2,
		QPSByChain:         make(map[string]float64),
		ConcurrencyByChain: make(map[string]int),
		RetryMaxAttempts:   3,
		RetryBaseDelay:     250 * time.Millisecond,
		RetryMaxDelay:      4 * time.Second,
	}
}

func mergeChainGatewayOptions(base, incoming ChainGatewayOptions) ChainGatewayOptions {
	if incoming.DefaultQPS > 0 {
		base.DefaultQPS = incoming.DefaultQPS
	}
	if incoming.DefaultConcurrency > 0 {
		base.DefaultConcurrency = incoming.DefaultConcurrency
	}
	if incoming.RetryMaxAttempts > 0 {
		base.RetryMaxAttempts = incoming.RetryMaxAttempts
	}
	if incoming.RetryBaseDelay > 0 {
		base.RetryBaseDelay = incoming.RetryBaseDelay
	}
	if incoming.RetryMaxDelay > 0 {
		base.RetryMaxDelay = incoming.RetryMaxDelay
	}
	if len(incoming.QPSByChain) > 0 {
		base.QPSByChain = cloneFloatMap(incoming.QPSByChain)
	}
	if len(incoming.ConcurrencyByChain) > 0 {
		base.ConcurrencyByChain = cloneIntMap(incoming.ConcurrencyByChain)
	}
	return base
}

func cloneFloatMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[strings.ToLower(strings.TrimSpace(k))] = v
	}
	return out
}

func cloneIntMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[strings.ToLower(strings.TrimSpace(k))] = v
	}
	return out
}

func withChainControl[T any](c *ChainGateway, ctx context.Context, chain string, op func(context.Context) (T, error)) (T, error) {
	var zero T
	gate := c.gateForChain(chain)
	release, err := gate.acquire(ctx)
	if err != nil {
		return zero, err
	}
	defer release()

	attempts := c.opts.RetryMaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if err := gate.waitTurn(ctx); err != nil {
			return zero, err
		}
		out, err := op(ctx)
		if err == nil {
			return out, nil
		}
		if attempt == attempts-1 || !isRetriableChainGatewayError(err) {
			return zero, err
		}
		if err := sleepWithContext(ctx, c.retryDelay(attempt)); err != nil {
			return zero, err
		}
	}
	return zero, nil
}

func (c *ChainGateway) retryDelay(attempt int) time.Duration {
	base := c.opts.RetryBaseDelay
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	maxDelay := c.opts.RetryMaxDelay
	if maxDelay <= 0 {
		maxDelay = 4 * time.Second
	}
	if maxDelay < base {
		maxDelay = base
	}
	multiplier := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(base) * multiplier)
	if delay > maxDelay {
		delay = maxDelay
	}
	jitterCap := delay / 2
	if jitterCap <= 0 {
		jitterCap = base / 2
	}
	if jitterCap > 0 {
		delay += time.Duration(rand.Int63n(jitterCap.Nanoseconds() + 1))
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func (c *ChainGateway) gateForChain(chain string) *chainRequestGate {
	key := strings.ToLower(strings.TrimSpace(chain))
	if key == "" {
		key = "default"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if gate, ok := c.gates[key]; ok {
		return gate
	}
	concurrency := c.opts.DefaultConcurrency
	if v, ok := c.opts.ConcurrencyByChain[key]; ok && v > 0 {
		concurrency = v
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	qps := c.opts.DefaultQPS
	if v, ok := c.opts.QPSByChain[key]; ok && v > 0 {
		qps = v
	}
	interval := time.Duration(0)
	if qps > 0 {
		interval = time.Duration(float64(time.Second) / qps)
	}
	gate := &chainRequestGate{
		sem:      make(chan struct{}, concurrency),
		interval: interval,
	}
	c.gates[key] = gate
	return gate
}

func (g *chainRequestGate) acquire(ctx context.Context) (func(), error) {
	select {
	case g.sem <- struct{}{}:
		return func() { <-g.sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *chainRequestGate) waitTurn(ctx context.Context) error {
	if g == nil || g.interval <= 0 {
		return nil
	}
	g.mu.Lock()
	now := time.Now()
	target := g.next
	if target.IsZero() || !target.After(now) {
		g.next = now.Add(g.interval)
		g.mu.Unlock()
		return nil
	}
	g.next = target.Add(g.interval)
	g.mu.Unlock()
	return sleepWithContext(ctx, time.Until(target))
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isRetriableChainGatewayError(err error) bool {
	if err == nil {
		return false
	}
	switch status.Code(err) {
	case codes.ResourceExhausted, codes.Unavailable, codes.DeadlineExceeded, codes.Aborted:
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "resource exhausted") ||
		strings.Contains(msg, "unavailable") ||
		strings.Contains(msg, "temporarily unavailable") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "eof")
}
