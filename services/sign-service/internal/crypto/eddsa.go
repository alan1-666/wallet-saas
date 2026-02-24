package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
)

func CreateEdDSAKeyPair() (string, string, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(privateKey), hex.EncodeToString(publicKey), nil
}

func SignEdDSAMessage(privateKeyHex string, messageHashHex string) (string, error) {
	privateKey, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", err
	}
	message, err := hex.DecodeString(messageHashHex)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(privateKey, message)
	return hex.EncodeToString(sig), nil
}
