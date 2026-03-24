package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/scrypt"
)

var vaultEnvelopePrefix = []byte("sv1")

func encryptSeed(password, storageKey string, seed []byte) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key, err := deriveVaultKey(password, storageKey, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, seed, []byte(storageKey))
	out := make([]byte, 0, len(vaultEnvelopePrefix)+len(salt)+len(nonce)+len(ciphertext))
	out = append(out, vaultEnvelopePrefix...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

func decryptSeed(password, storageKey string, payload []byte) ([]byte, error) {
	if len(payload) < len(vaultEnvelopePrefix)+16+12 {
		return nil, fmt.Errorf("invalid encrypted vault payload")
	}
	if string(payload[:len(vaultEnvelopePrefix)]) != string(vaultEnvelopePrefix) {
		return nil, fmt.Errorf("unsupported vault payload version")
	}
	saltOffset := len(vaultEnvelopePrefix)
	nonceOffset := saltOffset + 16
	ciphertextOffset := nonceOffset + 12
	salt := payload[saltOffset:nonceOffset]
	nonce := payload[nonceOffset:ciphertextOffset]
	ciphertext := payload[ciphertextOffset:]

	key, err := deriveVaultKey(password, storageKey, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(storageKey))
	if err != nil {
		return nil, fmt.Errorf("vault unlock failed: %w", err)
	}
	return plain, nil
}

func deriveVaultKey(password, storageKey string, salt []byte) ([]byte, error) {
	domain := sha256.Sum256([]byte(storageKey))
	derivedSalt := append(append([]byte(nil), salt...), domain[:]...)
	return scrypt.Key([]byte(password), derivedSalt, 1<<15, 8, 1, 32)
}
