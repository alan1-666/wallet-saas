package tron

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	sourcepb "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/rpc/account"
	common2 "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/rpc/common"
)

func TestDecodeUnsignedPayloadAcceptsStructuredJSON(t *testing.T) {
	raw, err := json.Marshal(TxStructure{
		ContractAddress: "TContract",
		FromAddress:     "TFrom",
		ToAddress:       "TTo",
		Value:           123,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	got, err := decodeUnsignedPayload(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatalf("decodeUnsignedPayload returned error: %v", err)
	}
	if got.ContractAddress != "TContract" || got.FromAddress != "TFrom" || got.ToAddress != "TTo" || got.Value != 123 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestDecodeUnsignedPayloadAcceptsLegacyFormat(t *testing.T) {
	got, err := decodeUnsignedPayload(base64.StdEncoding.EncodeToString([]byte("tron:TFrom:TTo:456")))
	if err != nil {
		t.Fatalf("decodeUnsignedPayload returned error: %v", err)
	}
	if got.FromAddress != "TFrom" || got.ToAddress != "TTo" || got.Value != 456 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestSignHashFromTransactionPayloadFallsBackToRawDataHex(t *testing.T) {
	unsignedTx, err := encodeTransactionBase64(&Transaction{RawDataHex: "0102"})
	if err != nil {
		t.Fatalf("encodeTransactionBase64 returned error: %v", err)
	}
	got, err := signHashFromTransactionPayload(unsignedTx)
	if err != nil {
		t.Fatalf("signHashFromTransactionPayload returned error: %v", err)
	}
	sum := sha256.Sum256([]byte{0x01, 0x02})
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("unexpected sign hash: got=%s want=%s", got, want)
	}
}

func TestBuildSignedTransactionAppendsSignature(t *testing.T) {
	unsignedTx, err := encodeTransactionBase64(&Transaction{
		TxID:       "deadbeef",
		RawDataHex: "0102",
	})
	if err != nil {
		t.Fatalf("encodeTransactionBase64 returned error: %v", err)
	}

	resp, err := (&ChainAdaptor{}).BuildSignedTransaction(&sourcepb.SignedTransactionRequest{
		Base64Tx:  unsignedTx,
		Signature: "0x" + strings.Repeat("11", 65),
	})
	if err != nil {
		t.Fatalf("BuildSignedTransaction returned error: %v", err)
	}
	if resp.GetCode() != common2.ReturnCode_SUCCESS {
		t.Fatalf("unexpected response code: %v msg=%s", resp.GetCode(), resp.GetMsg())
	}

	signedTx, err := decodeTransactionBase64(resp.GetSignedTx())
	if err != nil {
		t.Fatalf("decodeTransactionBase64 returned error: %v", err)
	}
	if len(signedTx.Signature) != 1 || signedTx.Signature[0] != strings.Repeat("11", 65) {
		t.Fatalf("unexpected signed transaction: %+v", signedTx)
	}
}
