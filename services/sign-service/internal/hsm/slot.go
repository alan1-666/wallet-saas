package hsm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func BuildTenantSlotID(slotPrefix, tenantID, signType string) string {
	slotPrefix = strings.TrimSpace(slotPrefix)
	if slotPrefix == "" {
		slotPrefix = "master"
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	safeTenant := tenantSlotKey(tenantID)
	return slotPrefix + ":" + safeTenant + ":" + strings.TrimSpace(signType)
}

func tenantSlotKey(tenantID string) string {
	if isSafeSlotID(tenantID) {
		return tenantID
	}
	h := sha256.Sum256([]byte(tenantID))
	return fmt.Sprintf("t_%s", hex.EncodeToString(h[:16]))
}

func isSafeSlotID(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
