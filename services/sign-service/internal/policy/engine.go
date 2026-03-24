package policy

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"wallet-saas-v2/services/sign-service/internal/hd"
)

type Config struct {
	AuthToken            string
	RateLimitWindow      time.Duration
	RateLimitMaxRequests int
	AllowedTenants       []string
}

type Decision struct {
	TokenLabel string
	TenantID   string
	KeyID      string
	SignType   string
	Operation  string
}

type Engine struct {
	authToken            string
	rateLimitWindow      time.Duration
	rateLimitMaxRequests int
	allowedTenants       map[string]struct{}

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	start time.Time
	count int
}

type auditEvent struct {
	At        string `json:"at"`
	Allowed   bool   `json:"allowed"`
	Operation string `json:"operation"`
	TenantID  string `json:"tenant_id,omitempty"`
	SignType  string `json:"sign_type,omitempty"`
	KeyID     string `json:"key_id,omitempty"`
	Token     string `json:"token"`
	Reason    string `json:"reason,omitempty"`
}

func New(cfg Config) *Engine {
	window := cfg.RateLimitWindow
	if window <= 0 {
		window = time.Minute
	}
	maxReq := cfg.RateLimitMaxRequests
	if maxReq <= 0 {
		maxReq = 300
	}
	return &Engine{
		authToken:            strings.TrimSpace(cfg.AuthToken),
		rateLimitWindow:      window,
		rateLimitMaxRequests: maxReq,
		allowedTenants:       makeAllowedTenants(cfg.AllowedTenants),
		buckets:              make(map[string]*bucket),
	}
}

func (e *Engine) Authorize(ctx context.Context, operation, signType, keyID string) (decision Decision, err error) {
	decision = Decision{
		Operation: strings.TrimSpace(operation),
		KeyID:     strings.TrimSpace(keyID),
		SignType:  normalizeSignType(signType),
		TenantID:  tenantFromContext(ctx),
	}
	defer func() {
		e.audit(decision, err == nil, err)
	}()

	token := tokenFromContext(ctx)
	decision.TokenLabel = maskToken(token)
	if e.authToken != "" && token != e.authToken {
		err = status.Error(codes.PermissionDenied, "sign service auth failed")
		return decision, err
	}
	if decision.Operation == "" {
		err = status.Error(codes.InvalidArgument, "operation is required")
		return decision, err
	}
	if decision.Operation == "derive" || decision.Operation == "sign" {
		if decision.TenantID == "" {
			err = status.Error(codes.InvalidArgument, "tenant id is required")
			return decision, err
		}
		if len(e.allowedTenants) > 0 {
			if _, ok := e.allowedTenants[decision.TenantID]; !ok {
				err = status.Error(codes.PermissionDenied, "tenant is not allowed by signer policy")
				return decision, err
			}
		}
		if decision.KeyID == "" {
			err = status.Error(codes.InvalidArgument, "key_id is required")
			return decision, err
		}
		if !strings.HasPrefix(strings.ToLower(decision.KeyID), "hd:") {
			err = status.Error(codes.PermissionDenied, "legacy key material is disabled")
			return decision, err
		}
		ref, parseErr := hd.ParseKeyID(decision.KeyID)
		if parseErr != nil {
			err = status.Error(codes.InvalidArgument, parseErr.Error())
			return decision, err
		}
		if decision.SignType != "" && decision.SignType != ref.SignType {
			err = status.Error(codes.InvalidArgument, "sign type mismatch")
			return decision, err
		}
		decision.SignType = ref.SignType
	}
	if rateErr := e.allow(token, decision.TenantID, decision.Operation); rateErr != nil {
		err = rateErr
		return decision, err
	}
	return decision, nil
}

func (e *Engine) allow(token, tenantID, operation string) error {
	key := strings.TrimSpace(token) + "|" + strings.TrimSpace(tenantID) + "|" + strings.TrimSpace(operation)
	now := time.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	b := e.buckets[key]
	if b == nil || now.Sub(b.start) >= e.rateLimitWindow {
		e.buckets[key] = &bucket{start: now, count: 1}
		return nil
	}
	if b.count >= e.rateLimitMaxRequests {
		return status.Error(codes.ResourceExhausted, "sign service rate limit exceeded")
	}
	b.count++
	return nil
}

func (e *Engine) audit(decision Decision, allowed bool, err error) {
	event := auditEvent{
		At:        time.Now().UTC().Format(time.RFC3339),
		Allowed:   allowed,
		Operation: decision.Operation,
		TenantID:  decision.TenantID,
		SignType:  decision.SignType,
		KeyID:     decision.KeyID,
		Token:     decision.TokenLabel,
	}
	if err != nil {
		event.Reason = err.Error()
	}
	raw, marshalErr := json.Marshal(event)
	if marshalErr != nil {
		log.Printf("sign policy audit marshal failed op=%s err=%v", decision.Operation, marshalErr)
		return
	}
	log.Printf("sign_policy %s", string(raw))
}

func metadataValueFromContext(ctx context.Context, keys ...string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, key := range keys {
		values := md.Get(key)
		if len(values) == 0 {
			continue
		}
		token := strings.TrimSpace(values[0])
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = strings.TrimSpace(token[7:])
		}
		if token != "" {
			return token
		}
	}
	return ""
}

func tokenFromContext(ctx context.Context) string {
	return metadataValueFromContext(ctx, "authorization", "x-sign-token")
}

func tenantFromContext(ctx context.Context) string {
	return strings.TrimSpace(metadataValueFromContext(ctx, "x-tenant-id", "tenant-id", "x-tenant"))
}

func maskToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "anonymous"
	}
	if len(token) <= 6 {
		return token
	}
	return token[:3] + "***" + token[len(token)-3:]
}

func normalizeSignType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ecdsa":
		return "ecdsa"
	case "eddsa", "ed25519":
		return "eddsa"
	default:
		return ""
	}
}

func makeAllowedTenants(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
