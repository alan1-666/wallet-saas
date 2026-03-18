package registry

import "fmt"

func buildHDKeyID(signType, chain string, accountIndex, changeIndex, addressIndex int64) string {
	return fmt.Sprintf("hd:%s:%s:%d:%d:%d", normalizeSignType(signType), normalizeChain(chain), accountIndex, changeIndex, addressIndex)
}

func buildHDDerivationPath(signType, chain string, accountIndex, changeIndex, addressIndex int64) (string, error) {
	coinType, err := hdCoinTypeForChain(chain)
	if err != nil {
		return "", err
	}
	switch normalizeSignType(signType) {
	case "ecdsa":
		return fmt.Sprintf("m/44'/%d'/%d'/%d/%d", coinType, accountIndex, changeIndex, addressIndex), nil
	case "eddsa":
		return fmt.Sprintf("m/44'/%d'/%d'/%d'/%d'", coinType, accountIndex, changeIndex, addressIndex), nil
	default:
		return "", fmt.Errorf("unsupported sign type: %s", signType)
	}
}

func hdCoinTypeForChain(chain string) (int64, error) {
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
	case "xlm", "stellar":
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
		return 0, fmt.Errorf("unsupported chain for hd path: %s", chain)
	}
}

func normalizeSignType(v string) string {
	switch v = normalizeChainSignType(v); v {
	case "ecdsa", "eddsa":
		return v
	default:
		return ""
	}
}

func normalizeChainSignType(v string) string {
	switch normalizeNetwork(v) {
	case "ecdsa":
		return "ecdsa"
	case "ed25519", "eddsa":
		return "eddsa"
	default:
		return ""
	}
}
