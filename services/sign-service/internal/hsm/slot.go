package hsm

import "strings"

func BuildTenantSlotID(slotPrefix, tenantID, signType string) string {
	slotPrefix = strings.TrimSpace(slotPrefix)
	if slotPrefix == "" {
		slotPrefix = "master"
	}
	tenantID = sanitizeSlotPart(tenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	return slotPrefix + ":" + tenantID + ":" + strings.TrimSpace(signType)
}

func sanitizeSlotPart(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(v))
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
