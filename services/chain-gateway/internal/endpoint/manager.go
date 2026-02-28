package endpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/controlplane"
)

type SelectedEndpoint struct {
	ID        int64
	Key       string
	Chain     string
	Network   string
	Model     string
	URL       string
	TimeoutMS int
}

type EndpointStatus struct {
	Chain        string `json:"chain"`
	Network      string `json:"network"`
	Model        string `json:"model"`
	URL          string `json:"url"`
	CircuitOpen  bool   `json:"circuit_open"`
	OpenUntil    string `json:"open_until,omitempty"`
	FailStreak   int    `json:"fail_streak"`
	SuccessCount int64  `json:"success_count"`
	FailureCount int64  `json:"failure_count"`
	LastError    string `json:"last_error,omitempty"`
}

type Manager struct {
	store          controlplane.RPCEndpointStore
	refreshEvery   time.Duration
	probeEvery     time.Duration
	openDuration   time.Duration
	failThreshold  int
	httpClient     *http.Client
	mu             sync.RWMutex
	pools          map[string][]*state
	states         map[string]*state
	rr             *rand.Rand
	lastRefreshErr error
}

type state struct {
	endpoint      controlplane.RPCEndpoint
	failStreak    int
	penalty       int
	openUntil     time.Time
	lastError     string
	lastSuccessAt time.Time
	lastFailureAt time.Time
	successCount  int64
	failureCount  int64
}

func NewManager(store controlplane.RPCEndpointStore, refreshEvery, probeEvery, openDuration time.Duration, failThreshold int) *Manager {
	if refreshEvery <= 0 {
		refreshEvery = 15 * time.Second
	}
	if probeEvery <= 0 {
		probeEvery = 20 * time.Second
	}
	if openDuration <= 0 {
		openDuration = 30 * time.Second
	}
	if failThreshold <= 0 {
		failThreshold = 3
	}
	return &Manager{
		store:         store,
		refreshEvery:  refreshEvery,
		probeEvery:    probeEvery,
		openDuration:  openDuration,
		failThreshold: failThreshold,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		pools:         make(map[string][]*state),
		states:        make(map[string]*state),
		rr:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *Manager) Run(ctx context.Context) {
	if m == nil || m.store == nil {
		return
	}
	if err := m.Refresh(ctx); err != nil {
		log.Printf("chain-gateway endpoint refresh failed: %v", err)
	}
	refreshTicker := time.NewTicker(m.refreshEvery)
	defer refreshTicker.Stop()
	probeTicker := time.NewTicker(m.probeEvery)
	defer probeTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-refreshTicker.C:
			if err := m.Refresh(ctx); err != nil {
				log.Printf("chain-gateway endpoint refresh failed: %v", err)
			}
		case <-probeTicker.C:
			m.probe(ctx)
		}
	}
}

func (m *Manager) Refresh(ctx context.Context) error {
	if m == nil || m.store == nil {
		return nil
	}
	items, err := m.store.ListActiveRPCEndpoints(ctx)
	if err != nil {
		m.mu.Lock()
		m.lastRefreshErr = err
		m.mu.Unlock()
		return err
	}

	newPools := make(map[string][]*state)
	newStates := make(map[string]*state)

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range items {
		k := endpointKey(item)
		st, ok := m.states[k]
		if !ok {
			st = &state{endpoint: item}
		} else {
			st.endpoint = item
		}
		newStates[k] = st
		poolKey := poolKey(item.Chain, item.Network, item.Model)
		newPools[poolKey] = append(newPools[poolKey], st)
	}
	m.pools = newPools
	m.states = newStates
	m.lastRefreshErr = nil
	return nil
}

func (m *Manager) Select(chain, network, model string) (SelectedEndpoint, error) {
	if m == nil {
		return SelectedEndpoint{}, fmt.Errorf("endpoint manager is nil")
	}
	chain = strings.ToLower(strings.TrimSpace(chain))
	network = strings.ToLower(strings.TrimSpace(network))
	model = strings.ToLower(strings.TrimSpace(model))
	if chain == "" || network == "" || model == "" {
		return SelectedEndpoint{}, fmt.Errorf("chain/network/model are required")
	}

	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	candidates := m.pools[poolKey(chain, network, model)]
	if len(candidates) == 0 {
		return SelectedEndpoint{}, fmt.Errorf("no rpc endpoint configured for chain=%s network=%s model=%s", chain, network, model)
	}

	available := make([]*state, 0, len(candidates))
	for _, st := range candidates {
		if st.openUntil.IsZero() || !now.Before(st.openUntil) {
			available = append(available, st)
		}
	}
	if len(available) == 0 {
		// Circuit is open for all endpoints: pick the soonest half-open candidate.
		var pick *state
		for _, st := range candidates {
			if pick == nil || st.openUntil.Before(pick.openUntil) {
				pick = st
			}
		}
		return toSelected(pick), nil
	}

	totalWeight := 0
	for _, st := range available {
		totalWeight += effectiveWeight(st)
	}
	if totalWeight <= 0 {
		return toSelected(available[0]), nil
	}
	r := m.rr.Intn(totalWeight)
	acc := 0
	for _, st := range available {
		acc += effectiveWeight(st)
		if r < acc {
			return toSelected(st), nil
		}
	}
	return toSelected(available[0]), nil
}

func (m *Manager) ReportSuccess(endpointKey string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.states[endpointKey]
	if !ok {
		return
	}
	st.failStreak = 0
	st.lastError = ""
	st.lastSuccessAt = time.Now()
	st.successCount++
	if st.penalty > 0 {
		st.penalty--
	}
	st.openUntil = time.Time{}
}

func (m *Manager) ReportFailure(endpointKey string, err error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.states[endpointKey]
	if !ok {
		return
	}
	st.failStreak++
	st.failureCount++
	st.lastFailureAt = time.Now()
	if err != nil {
		st.lastError = err.Error()
	}
	maxPenalty := st.endpoint.Weight - 1
	if maxPenalty < 0 {
		maxPenalty = 0
	}
	if st.penalty < maxPenalty {
		st.penalty++
	}
	if st.failStreak >= m.failThreshold {
		st.openUntil = time.Now().Add(m.openDuration)
		st.failStreak = 0
	}
}

func (m *Manager) probe(ctx context.Context) {
	snap := m.snapshot()
	for _, item := range snap {
		if item.Model != "account" || !clients.IsEVMChain(item.Chain) {
			continue
		}
		if err := probeJSONRPC(ctx, m.httpClient, item.URL, item.TimeoutMS); err != nil {
			m.ReportFailure(item.Key, err)
			log.Printf("chain-gateway endpoint probe failed chain=%s network=%s model=%s url=%s err=%v",
				item.Chain, item.Network, item.Model, item.URL, err)
			continue
		}
		m.ReportSuccess(item.Key)
	}
}

func (m *Manager) snapshot() []SelectedEndpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SelectedEndpoint, 0, len(m.states))
	for k, st := range m.states {
		x := toSelected(st)
		x.Key = k
		out = append(out, x)
	}
	return out
}

func (m *Manager) Snapshot() []EndpointStatus {
	if m == nil {
		return nil
	}
	now := time.Now()
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]EndpointStatus, 0, len(m.states))
	for _, st := range m.states {
		item := EndpointStatus{
			Chain:        st.endpoint.Chain,
			Network:      st.endpoint.Network,
			Model:        st.endpoint.Model,
			URL:          st.endpoint.URL,
			CircuitOpen:  !st.openUntil.IsZero() && now.Before(st.openUntil),
			FailStreak:   st.failStreak,
			SuccessCount: st.successCount,
			FailureCount: st.failureCount,
			LastError:    st.lastError,
		}
		if !st.openUntil.IsZero() {
			item.OpenUntil = st.openUntil.UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out
}

func toSelected(st *state) SelectedEndpoint {
	if st == nil {
		return SelectedEndpoint{}
	}
	return SelectedEndpoint{
		ID:        st.endpoint.ID,
		Key:       endpointKey(st.endpoint),
		Chain:     st.endpoint.Chain,
		Network:   st.endpoint.Network,
		Model:     st.endpoint.Model,
		URL:       st.endpoint.URL,
		TimeoutMS: st.endpoint.TimeoutMS,
	}
}

func endpointKey(ep controlplane.RPCEndpoint) string {
	return ep.Chain + "|" + ep.Network + "|" + ep.Model + "|" + ep.URL
}

func poolKey(chain, network, model string) string {
	return chain + "|" + network + "|" + model
}

func effectiveWeight(st *state) int {
	if st == nil {
		return 0
	}
	w := st.endpoint.Weight - st.penalty
	if w <= 0 {
		return 1
	}
	return w
}

func probeJSONRPC(ctx context.Context, hc *http.Client, url string, timeoutMS int) error {
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}
	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "web3_clientVersion",
		"params":  []any{},
	}
	raw, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d", resp.StatusCode)
	}
	return nil
}
