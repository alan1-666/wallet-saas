package hd

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/big"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type DerivedKey struct {
	KeyID              string
	DerivationPath     string
	PublicKeyHex       string
	AlternatePublicKey string
	PrivateKeyHex      string
}

func DeriveKey(masterSeed []byte, ref KeyRef) (DerivedKey, error) {
	path, err := ref.pathIndexes()
	if err != nil {
		return DerivedKey{}, err
	}
	derivationPath, err := ref.DerivationPath()
	if err != nil {
		return DerivedKey{}, err
	}
	switch ref.SignType {
	case "ecdsa":
		priv, pub, compressed, err := deriveECDSA(masterSeed, path)
		if err != nil {
			return DerivedKey{}, err
		}
		return DerivedKey{
			KeyID:              ref.ID(),
			DerivationPath:     derivationPath,
			PublicKeyHex:       pub,
			AlternatePublicKey: compressed,
			PrivateKeyHex:      priv,
		}, nil
	case "eddsa":
		priv, pub, err := deriveEdDSA(masterSeed, path)
		if err != nil {
			return DerivedKey{}, err
		}
		return DerivedKey{
			KeyID:              ref.ID(),
			DerivationPath:     derivationPath,
			PublicKeyHex:       pub,
			AlternatePublicKey: pub,
			PrivateKeyHex:      priv,
		}, nil
	default:
		return DerivedKey{}, fmt.Errorf("unsupported sign type: %s", ref.SignType)
	}
}

func deriveECDSA(masterSeed []byte, path []uint32) (string, string, string, error) {
	key, chainCode, err := secpMaster(masterSeed)
	if err != nil {
		return "", "", "", err
	}
	for _, index := range path {
		key, chainCode, err = secpChild(key, chainCode, index)
		if err != nil {
			return "", "", "", err
		}
	}
	privKey, err := gethcrypto.ToECDSA(key)
	if err != nil {
		return "", "", "", err
	}
	privHex := hex.EncodeToString(gethcrypto.FromECDSA(privKey))
	pubHex := hex.EncodeToString(gethcrypto.FromECDSAPub(&privKey.PublicKey))
	compressedHex := hex.EncodeToString(gethcrypto.CompressPubkey(&privKey.PublicKey))
	return privHex, pubHex, compressedHex, nil
}

func secpMaster(seed []byte) ([]byte, []byte, error) {
	sum := hmacSHA512([]byte("Bitcoin seed"), seed)
	key := append([]byte(nil), sum[:32]...)
	chainCode := append([]byte(nil), sum[32:]...)
	if err := validateSecpKey(key); err != nil {
		return nil, nil, err
	}
	return key, chainCode, nil
}

func secpChild(parentKey, parentChainCode []byte, index uint32) ([]byte, []byte, error) {
	if len(parentKey) != 32 || len(parentChainCode) != 32 {
		return nil, nil, fmt.Errorf("invalid parent key state")
	}
	data := make([]byte, 0, 37)
	if index >= hardenedOffset {
		data = append(data, 0x00)
		data = append(data, parentKey...)
	} else {
		privKey, err := gethcrypto.ToECDSA(parentKey)
		if err != nil {
			return nil, nil, err
		}
		data = append(data, gethcrypto.CompressPubkey(&privKey.PublicKey)...)
	}
	data = appendUint32(data, index)
	sum := hmacSHA512(parentChainCode, data)
	parsed := new(big.Int).SetBytes(sum[:32])
	curveN := gethcrypto.S256().Params().N
	if parsed.Sign() == 0 || parsed.Cmp(curveN) >= 0 {
		return nil, nil, fmt.Errorf("derived invalid secp256k1 child key")
	}
	parentInt := new(big.Int).SetBytes(parentKey)
	parsed.Add(parsed, parentInt)
	parsed.Mod(parsed, curveN)
	if parsed.Sign() == 0 {
		return nil, nil, fmt.Errorf("derived zero secp256k1 child key")
	}
	childKey := leftPad(parsed.Bytes(), 32)
	childChainCode := append([]byte(nil), sum[32:]...)
	return childKey, childChainCode, nil
}

func validateSecpKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("invalid secp256k1 key length")
	}
	keyInt := new(big.Int).SetBytes(key)
	curveN := gethcrypto.S256().Params().N
	if keyInt.Sign() == 0 || keyInt.Cmp(curveN) >= 0 {
		return fmt.Errorf("invalid secp256k1 private key")
	}
	return nil
}

func deriveEdDSA(masterSeed []byte, path []uint32) (string, string, error) {
	seed, chainCode := edMaster(masterSeed)
	for _, index := range path {
		if index < hardenedOffset {
			return "", "", fmt.Errorf("ed25519 derivation only supports hardened children")
		}
		seed, chainCode = edChild(seed, chainCode, index)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)
	return hex.EncodeToString(privKey), hex.EncodeToString(pubKey), nil
}

func edMaster(seed []byte) ([]byte, []byte) {
	sum := hmacSHA512([]byte("ed25519 seed"), seed)
	return append([]byte(nil), sum[:32]...), append([]byte(nil), sum[32:]...)
}

func edChild(parentSeed, parentChainCode []byte, index uint32) ([]byte, []byte) {
	data := make([]byte, 0, 37)
	data = append(data, 0x00)
	data = append(data, parentSeed...)
	data = appendUint32(data, index)
	sum := hmacSHA512(parentChainCode, data)
	return append([]byte(nil), sum[:32]...), append([]byte(nil), sum[32:]...)
}

func hmacSHA512(key, data []byte) []byte {
	mac := hmac.New(sha512.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func appendUint32(dst []byte, v uint32) []byte {
	return append(dst, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func leftPad(src []byte, size int) []byte {
	if len(src) >= size {
		return append([]byte(nil), src[len(src)-size:]...)
	}
	out := make([]byte, size)
	copy(out[size-len(src):], src)
	return out
}
