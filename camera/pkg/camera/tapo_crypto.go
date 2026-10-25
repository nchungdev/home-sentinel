package camera

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func generateRandomHex(length int) (string, error) {
	bytesNeeded := (length + 1) / 2
	buffer := make([]byte, bytesNeeded)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer)[:length], nil
}

func sha256Hex(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

func md5Hex(content string) string {
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:])
}

func pkcs7Pad(input []byte, blockSize int) []byte {
	padding := blockSize - (len(input) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(input, padText...)
}

func pkcs7Unpad(input []byte) ([]byte, error) {
	length := len(input)
	if length == 0 {
		return nil, fmt.Errorf("dữ liệu giải mã rỗng")
	}
	padding := int(input[length-1])
	if padding > length || padding == 0 {
		return nil, fmt.Errorf("padding không hợp lệ")
	}
	for i := length - padding; i < length; i++ {
		if input[i] != byte(padding) {
			return nil, fmt.Errorf("padding byte không đồng nhất")
		}
	}
	return input[:length-padding], nil
}

func encryptAesCbc(plaintext []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(padded))
	mode.CryptBlocks(ciphertext, padded)
	return ciphertext, nil
}

func decryptAesCbc(ciphertext []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("độ dài ciphertext không phải bội số của block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)
	return pkcs7Unpad(plaintext)
}
