package hd

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	keyIDPrefix    = "hd"
	hardenedOffset = uint32(0x80000000)
)

type KeyRef struct {
	SignType string
	Chain    string
	Account  uint32
	Change   uint32
	Index    uint32
}

func BuildKeyID(signType, chain string, account, change, index uint32) string {
	return fmt.Sprintf("%s:%s:%s:%d:%d:%d", keyIDPrefix, normalizeSignType(signType), normalizeChain(chain), account, change, index)
}

func ParseKeyID(raw string) (KeyRef, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 6 {
		return KeyRef{}, fmt.Errorf("invalid hd key id")
	}
	if strings.ToLower(strings.TrimSpace(parts[0])) != keyIDPrefix {
		return KeyRef{}, fmt.Errorf("unsupported key id prefix")
	}
	signType := normalizeSignType(parts[1])
	chain := normalizeChain(parts[2])
	if signType == "" || chain == "" {
		return KeyRef{}, fmt.Errorf("invalid hd key id")
	}
	account, err := parseUint32(parts[3])
	if err != nil {
		return KeyRef{}, fmt.Errorf("invalid account index: %w", err)
	}
	change, err := parseUint32(parts[4])
	if err != nil {
		return KeyRef{}, fmt.Errorf("invalid change index: %w", err)
	}
	index, err := parseUint32(parts[5])
	if err != nil {
		return KeyRef{}, fmt.Errorf("invalid address index: %w", err)
	}
	return KeyRef{
		SignType: signType,
		Chain:    chain,
		Account:  account,
		Change:   change,
		Index:    index,
	}, nil
}

func (r KeyRef) ID() string {
	return BuildKeyID(r.SignType, r.Chain, r.Account, r.Change, r.Index)
}

func (r KeyRef) DerivationPath() (string, error) {
	coinType, err := coinTypeForChain(r.Chain)
	if err != nil {
		return "", err
	}
	switch r.SignType {
	case "ecdsa":
		return fmt.Sprintf("m/44'/%d'/%d'/%d/%d", coinType, r.Account, r.Change, r.Index), nil
	case "eddsa":
		return fmt.Sprintf("m/44'/%d'/%d'/%d'/%d'", coinType, r.Account, r.Change, r.Index), nil
	default:
		return "", fmt.Errorf("unsupported sign type: %s", r.SignType)
	}
}

func (r KeyRef) AccountDerivationPath() (string, error) {
	coinType, err := coinTypeForChain(r.Chain)
	if err != nil {
		return "", err
	}
	switch r.SignType {
	case "ecdsa", "eddsa":
		return fmt.Sprintf("m/44'/%d'/%d'", coinType, r.Account), nil
	default:
		return "", fmt.Errorf("unsupported sign type: %s", r.SignType)
	}
}

func (r KeyRef) pathIndexes() ([]uint32, error) {
	coinType, err := coinTypeForChain(r.Chain)
	if err != nil {
		return nil, err
	}
	switch r.SignType {
	case "ecdsa":
		return []uint32{
			44 + hardenedOffset,
			coinType + hardenedOffset,
			r.Account + hardenedOffset,
			r.Change,
			r.Index,
		}, nil
	case "eddsa":
		return []uint32{
			44 + hardenedOffset,
			coinType + hardenedOffset,
			r.Account + hardenedOffset,
			r.Change + hardenedOffset,
			r.Index + hardenedOffset,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported sign type: %s", r.SignType)
	}
}

func (r KeyRef) accountPathIndexes() ([]uint32, error) {
	coinType, err := coinTypeForChain(r.Chain)
	if err != nil {
		return nil, err
	}
	return []uint32{
		44 + hardenedOffset,
		coinType + hardenedOffset,
		r.Account + hardenedOffset,
	}, nil
}

func parseUint32(raw string) (uint32, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
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

func normalizeChain(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "eth":
		return "ethereum"
	case "bsc":
		return "binance"
	case "sol":
		return "solana"
	case "btc":
		return "bitcoin"
	case "ltc":
		return "litecoin"
	case "bch":
		return "bitcoincash"
	case "doge":
		return "dogecoin"
	case "xlm":
		return "stellar"
	case "":
		return ""
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func coinTypeForChain(chain string) (uint32, error) {
	switch normalizeChain(chain) {
	case "ethereum", "binance", "polygon", "arbitrum", "optimism", "linea", "scroll", "mantle", "zksync":
		return 60, nil
	case "tron":
		return 195, nil
	case "cosmos":
		return 118, nil
	case "btt":
		return 199, nil
	case "solana":
		return 501, nil
	case "aptos":
		return 637, nil
	case "sui":
		return 784, nil
	case "ton":
		return 607, nil
	case "stellar":
		return 148, nil
	case "bitcoin":
		return 0, nil
	case "litecoin":
		return 2, nil
	case "dogecoin":
		return 3, nil
	case "dash":
		return 5, nil
	case "zen":
		return 121, nil
	case "bitcoincash":
		return 145, nil
	default:
		return 0, fmt.Errorf("unsupported chain for hd key: %s", chain)
	}
}
