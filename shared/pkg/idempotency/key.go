package idempotency

import "fmt"

func BuildKey(tenantID, requestID string) string {
	return fmt.Sprintf("idem:%s:%s", tenantID, requestID)
}
