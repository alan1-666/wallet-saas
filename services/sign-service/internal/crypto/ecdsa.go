package crypto

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func CreateECDSAKeyPair() (string, string, string, error) {
	privateKey, err := gethcrypto.GenerateKey()
	if err != nil {
		return "", "", "", err
	}

	priKeyStr := hex.EncodeToString(gethcrypto.FromECDSA(privateKey))
	pubKeyStr := hex.EncodeToString(gethcrypto.FromECDSAPub(&privateKey.PublicKey))
	compressedPubkey := hex.EncodeToString(gethcrypto.CompressPubkey(&privateKey.PublicKey))
	return priKeyStr, pubKeyStr, compressedPubkey, nil
}

func SignECDSAMessage(privKeyHex string, messageHashHex string) (string, error) {
	hash := common.HexToHash(messageHashHex)
	privByte, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return "", err
	}
	privKey, err := gethcrypto.ToECDSA(privByte)
	if err != nil {
		return "", err
	}
	signature, err := gethcrypto.Sign(hash[:], privKey)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(signature), nil
}
