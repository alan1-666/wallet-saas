package hdpub

import (
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func DeriveECDSAChildPublic(accountCompressedHex, accountChainCodeHex string, changeIndex, addressIndex int64) (compressedHex, uncompressedHex string, err error) {
	if changeIndex < 0 || addressIndex < 0 {
		return "", "", fmt.Errorf("change/address index must be non-negative")
	}
	accountCompressed, err := decodeHex(accountCompressedHex)
	if err != nil {
		return "", "", fmt.Errorf("decode account public key: %w", err)
	}
	accountChainCode, err := decodeHex(accountChainCodeHex)
	if err != nil {
		return "", "", fmt.Errorf("decode account chain code: %w", err)
	}
	changePub, changeChainCode, err := secpChildPublic(accountCompressed, accountChainCode, uint32(changeIndex))
	if err != nil {
		return "", "", err
	}
	childPub, _, err := secpChildPublic(changePub, changeChainCode, uint32(addressIndex))
	if err != nil {
		return "", "", err
	}
	pub, err := parseSecpPublicKey(childPub)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(gethcrypto.CompressPubkey(pub)), hex.EncodeToString(gethcrypto.FromECDSAPub(pub)), nil
}

func secpChildPublic(parentCompressed, parentChainCode []byte, index uint32) ([]byte, []byte, error) {
	if len(parentCompressed) == 0 || len(parentChainCode) != 32 {
		return nil, nil, fmt.Errorf("invalid parent public key state")
	}
	data := append([]byte(nil), parentCompressed...)
	data = appendUint32(data, index)
	sum := hmacSHA512(parentChainCode, data)

	curve := gethcrypto.S256()
	curveN := curve.Params().N
	il := new(big.Int).SetBytes(sum[:32])
	if il.Sign() == 0 || il.Cmp(curveN) >= 0 {
		return nil, nil, fmt.Errorf("derived invalid secp256k1 child public key")
	}

	parentPub, err := parseSecpPublicKey(parentCompressed)
	if err != nil {
		return nil, nil, err
	}
	x1, y1 := curve.ScalarBaseMult(sum[:32])
	x, y := curve.Add(x1, y1, parentPub.X, parentPub.Y)
	if x == nil || y == nil {
		return nil, nil, fmt.Errorf("derived invalid secp256k1 child public key")
	}
	childPub := gethcrypto.CompressPubkey(&ecdsa.PublicKey{Curve: curve, X: x, Y: y})
	return childPub, append([]byte(nil), sum[32:]...), nil
}

func parseSecpPublicKey(pub []byte) (*ecdsa.PublicKey, error) {
	parsed, err := gethcrypto.DecompressPubkey(pub)
	if err == nil {
		return parsed, nil
	}
	return gethcrypto.UnmarshalPubkey(pub)
}

func hmacSHA512(key, data []byte) []byte {
	mac := hmac.New(sha512.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func appendUint32(dst []byte, v uint32) []byte {
	return append(dst, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func decodeHex(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "0x"), "0X")
	return hex.DecodeString(raw)
}
