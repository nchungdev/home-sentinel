package camstream

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

func generateRandomHex(length int) (string, error) {
	randomBytes := make([]byte, length)
	if _, readErr := rand.Read(randomBytes); readErr != nil {
		return "", readErr
	}

	return strings.ToUpper(hex.EncodeToString(randomBytes)), nil
}

func sha256Hex(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return strings.ToUpper(hex.EncodeToString(hasher.Sum(nil)))
}

func md5Hex(content string) string {
	hasher := md5.New()
	hasher.Write([]byte(content))
	return strings.ToUpper(hex.EncodeToString(hasher.Sum(nil)))
}

func pkcs7Pad(input []byte, blockSize int) []byte {
	paddingSize := blockSize - len(input)%blockSize
	paddingBytes := bytes.Repeat([]byte{byte(paddingSize)}, paddingSize)
	return append(input, paddingBytes...)
}

func pkcs7Unpad(input []byte) ([]byte, error) {
	length := len(input)
	if length == 0 {
		return nil, errors.New("empty ciphertext buffer")
	}

	paddingSize := int(input[length-1])
	if paddingSize > length || paddingSize == 0 {
		return nil, errors.New("invalid padding format")
	}

	return input[:length-paddingSize], nil
}

func encryptAesCbc(plaintext []byte, key []byte, iv []byte) ([]byte, error) {
	cipherBlock, cipherErr := aes.NewCipher(key)
	if cipherErr != nil {
		return nil, cipherErr
	}

	paddedData := pkcs7Pad(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(paddedData))
	encrypter := cipher.NewCBCEncrypter(cipherBlock, iv)
	encrypter.CryptBlocks(ciphertext, paddedData)

	return ciphertext, nil
}

func decryptAesCbc(ciphertext []byte, key []byte, iv []byte) ([]byte, error) {
	cipherBlock, cipherErr := aes.NewCipher(key)
	if cipherErr != nil {
		return nil, cipherErr
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not multiple of block size")
	}

	decryptedPadded := make([]byte, len(ciphertext))
	decrypter := cipher.NewCBCDecrypter(cipherBlock, iv)
	decrypter.CryptBlocks(decryptedPadded, ciphertext)

	return pkcs7Unpad(decryptedPadded)
}
