package tron

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/mr-tron/base58"
	"math/big"
	"strings"
)

const (
	AddressPrefix = "41"
)

// Base58ToHex Convert TRON address from base58 to hexadecimal

// Base58 转 Hex
func Base58ToHex(base58Addr string) (string, error) {

	// 解码 base58 地址
	dec, _ := base58.Decode(base58Addr)

	// 检查解码后的长度是否为 25 字节
	if len(dec) != 25 {
		panic("无效的长度")
	}

	// 提取初始地址（前 21 字节）
	initialAddress := dec[:21]

	// 计算验证代码
	expectedVerificationCode := make([]byte, 4)
	hash := sha256.Sum256(initialAddress)
	hash2 := sha256.Sum256(hash[:])
	copy(expectedVerificationCode, hash2[:4])

	// 验证验证代码
	if !bytes.Equal(dec[21:], expectedVerificationCode) {
		panic("无效的验证代码")
	}

	// 将初始地址转换为 hex 字符串，并添加 "0x" 前缀
	hexAddress := "0x" + hex.EncodeToString(initialAddress)
	return hexAddress, nil
}

// PadLeftZero Fill the left side of the hexadecimal string with zero to the specified length
func PadLeftZero(hexStr string, length int) string {
	return strings.Repeat("0", length-len(hexStr)) + hexStr
}

// ParseTRC20TransferData Extract the 'to' address and 'amount' from ABI encoded data`
func ParseTRC20TransferData(data string) (string, *big.Int) {
	// Extract the receiving address (10-20 bytes, 2 characters per byte in hexadecimal, positions 20 to 40)
	toAddressHex := data[32:72]
	toAddress := address.HexToAddress(AddressPrefix + toAddressHex) // TRON addresses usually start with '41'
	valueHex := data[72:136]                                        // Get amount
	value := new(big.Int)
	value.SetString(valueHex, 16) // Parse hexadecimal to integer
	return toAddress.String(), value
}
