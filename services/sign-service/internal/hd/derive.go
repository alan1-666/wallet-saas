package hd

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type DerivedKey struct {
	KeyID                     string
	DerivationPath            string
	PublicKeyHex              string
	AlternatePublicKey        string
	PublicDerivationSupported bool
	AccountPublicKeyHex       string
	AccountAlternatePublicKey string
	AccountChainCodeHex       string
	AccountDerivationPath     string
}

type SigningKey struct {
	KeyID          string
	DerivationPath string
	PrivateKeyHex  string
}

func DerivePublicKey(masterSeed []byte, ref KeyRef) (DerivedKey, error) {
	derivationPath, err := ref.DerivationPath()
	if err != nil {
		return DerivedKey{}, err
	}
	switch ref.SignType {
	case "ecdsa":
		return deriveECDSAPublic(masterSeed, ref, derivationPath)
	case "eddsa":
		pub, err := deriveEdDSAPublic(masterSeed, ref)
		if err != nil {
			return DerivedKey{}, err
		}
		return DerivedKey{
			KeyID:                     ref.ID(),
			DerivationPath:            derivationPath,
			PublicKeyHex:              pub,
			AlternatePublicKey:        pub,
			PublicDerivationSupported: false,
		}, nil
	default:
		return DerivedKey{}, fmt.Errorf("unsupported sign type: %s", ref.SignType)
	}
}

func DeriveSigningKey(masterSeed []byte, ref KeyRef) (SigningKey, error) {
	path, err := ref.pathIndexes()
	if err != nil {
		return SigningKey{}, err
	}
	derivationPath, err := ref.DerivationPath()
	if err != nil {
		return SigningKey{}, err
	}
	switch ref.SignType {
	case "ecdsa":
		priv, err := deriveECDSAPrivate(masterSeed, path)
		if err != nil {
			return SigningKey{}, err
		}
		return SigningKey{
			KeyID:          ref.ID(),
			DerivationPath: derivationPath,
			PrivateKeyHex:  priv,
		}, nil
	case "eddsa":
		priv, err := deriveEdDSAPrivate(masterSeed, path)
		if err != nil {
			return SigningKey{}, err
		}
		return SigningKey{
			KeyID:          ref.ID(),
			DerivationPath: derivationPath,
			PrivateKeyHex:  priv,
		}, nil
	default:
		return SigningKey{}, fmt.Errorf("unsupported sign type: %s", ref.SignType)
	}
}

func DeriveECDSAChildPublic(accountCompressedHex, accountChainCodeHex string, changeIndex, addressIndex uint32) (compressedHex, uncompressedHex string, err error) {
	accountCompressed, err := hex.DecodeString(stringsTrimHex(accountCompressedHex))
	if err != nil {
		return "", "", fmt.Errorf("decode account public key: %w", err)
	}
	accountChainCode, err := hex.DecodeString(stringsTrimHex(accountChainCodeHex))
	if err != nil {
		return "", "", fmt.Errorf("decode account chain code: %w", err)
	}
	changePub, changeChainCode, err := secpChildPublic(accountCompressed, accountChainCode, changeIndex)
	if err != nil {
		return "", "", err
	}
	childPub, _, err := secpChildPublic(changePub, changeChainCode, addressIndex)
	if err != nil {
		return "", "", err
	}
	pub, err := parseSecpPublicKey(childPub)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(gethcrypto.CompressPubkey(pub)), hex.EncodeToString(gethcrypto.FromECDSAPub(pub)), nil
}

func deriveECDSAPublic(masterSeed []byte, ref KeyRef, derivationPath string) (DerivedKey, error) {
	accountPath, err := ref.accountPathIndexes()
	if err != nil {
		return DerivedKey{}, err
	}
	accountDerivationPath, err := ref.AccountDerivationPath()
	if err != nil {
		return DerivedKey{}, err
	}

	accountKey, accountChainCode, err := deriveECDSAKeyState(masterSeed, accountPath)
	if err != nil {
		return DerivedKey{}, err
	}
	accountPriv, err := gethcrypto.ToECDSA(accountKey)
	if err != nil {
		return DerivedKey{}, err
	}
	accountCompressed := gethcrypto.CompressPubkey(&accountPriv.PublicKey)
	accountUncompressed := gethcrypto.FromECDSAPub(&accountPriv.PublicKey)

	childCompressed, childUncompressed, err := deriveECDSAChildFromAccount(accountCompressed, accountChainCode, ref.Change, ref.Index)
	if err != nil {
		return DerivedKey{}, err
	}

	return DerivedKey{
		KeyID:                     ref.ID(),
		DerivationPath:            derivationPath,
		PublicKeyHex:              childCompressed,
		AlternatePublicKey:        childUncompressed,
		PublicDerivationSupported: true,
		AccountPublicKeyHex:       hex.EncodeToString(accountCompressed),
		AccountAlternatePublicKey: hex.EncodeToString(accountUncompressed),
		AccountChainCodeHex:       hex.EncodeToString(accountChainCode),
		AccountDerivationPath:     accountDerivationPath,
	}, nil
}

func deriveECDSAChildFromAccount(accountCompressed, accountChainCode []byte, changeIndex, addressIndex uint32) (compressedHex, uncompressedHex string, err error) {
	changePub, changeChainCode, err := secpChildPublic(accountCompressed, accountChainCode, changeIndex)
	if err != nil {
		return "", "", err
	}
	childPub, _, err := secpChildPublic(changePub, changeChainCode, addressIndex)
	if err != nil {
		return "", "", err
	}
	pub, err := parseSecpPublicKey(childPub)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(gethcrypto.CompressPubkey(pub)), hex.EncodeToString(gethcrypto.FromECDSAPub(pub)), nil
}

func deriveECDSAKeyState(masterSeed []byte, path []uint32) ([]byte, []byte, error) {
	key, chainCode, err := secpMaster(masterSeed)
	if err != nil {
		return nil, nil, err
	}
	for _, index := range path {
		key, chainCode, err = secpChild(key, chainCode, index)
		if err != nil {
			return nil, nil, err
		}
	}
	return key, chainCode, nil
}

func deriveECDSAPrivate(masterSeed []byte, path []uint32) (string, error) {
	key, _, err := deriveECDSAKeyState(masterSeed, path)
	if err != nil {
		return "", err
	}
	privKey, err := gethcrypto.ToECDSA(key)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(gethcrypto.FromECDSA(privKey)), nil
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

func secpChildPublic(parentCompressed, parentChainCode []byte, index uint32) ([]byte, []byte, error) {
	if len(parentCompressed) == 0 || len(parentChainCode) != 32 {
		return nil, nil, fmt.Errorf("invalid parent public key state")
	}
	if index >= hardenedOffset {
		return nil, nil, fmt.Errorf("public derivation does not support hardened child index")
	}
	data := append([]byte(nil), parentCompressed...)
	data = appendUint32(data, index)
	sum := hmacSHA512(parentChainCode, data)
	il := new(big.Int).SetBytes(sum[:32])
	curve := gethcrypto.S256()
	curveN := curve.Params().N
	if il.Sign() == 0 || il.Cmp(curveN) >= 0 {
		return nil, nil, fmt.Errorf("derived invalid secp256k1 child public key")
	}

	parentPub, err := parseSecpPublicKey(parentCompressed)
	if err != nil {
		return nil, nil, err
	}
	x1, y1 := curve.ScalarBaseMult(sum[:32])
	x2, y2 := parentPub.X, parentPub.Y
	x, y := curve.Add(x1, y1, x2, y2)
	if x == nil || y == nil {
		return nil, nil, fmt.Errorf("derived invalid secp256k1 child public key")
	}
	childPub := gethcrypto.CompressPubkey(&ecdsa.PublicKey{Curve: gethcrypto.S256(), X: x, Y: y})
	return childPub, append([]byte(nil), sum[32:]...), nil
}

func parseSecpPublicKey(pub []byte) (*ecdsa.PublicKey, error) {
	parsed, err := gethcrypto.DecompressPubkey(pub)
	if err == nil {
		return parsed, nil
	}
	parsed, err = gethcrypto.UnmarshalPubkey(pub)
	if err != nil {
		return nil, err
	}
	return parsed, nil
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

func deriveEdDSAPublic(masterSeed []byte, ref KeyRef) (string, error) {
	path, err := ref.pathIndexes()
	if err != nil {
		return "", err
	}
	seed, chainCode := edMaster(masterSeed)
	for _, index := range path {
		if index < hardenedOffset {
			return "", fmt.Errorf("ed25519 derivation only supports hardened children")
		}
		seed, chainCode = edChild(seed, chainCode, index)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)
	return hex.EncodeToString(pubKey), nil
}

func deriveEdDSAPrivate(masterSeed []byte, path []uint32) (string, error) {
	seed, chainCode := edMaster(masterSeed)
	for _, index := range path {
		if index < hardenedOffset {
			return "", fmt.Errorf("ed25519 derivation only supports hardened children")
		}
		seed, chainCode = edChild(seed, chainCode, index)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	return hex.EncodeToString(privKey), nil
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

func stringsTrimHex(raw string) string {
	raw = strings.TrimSpace(raw)
	return strings.TrimPrefix(strings.TrimPrefix(raw, "0x"), "0X")
}
