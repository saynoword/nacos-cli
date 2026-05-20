package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	encryptionKeyFile    = "key"
	encryptedValuePrefix = "ENC[v1:aes-256-gcm:"
	encryptedValueSuffix = "]"
	encryptionKeySize    = 32
	gcmNonceSize         = 12
)

func encryptValue(plaintext string) (string, error) {
	if plaintext == "" || isEncryptedValue(plaintext) {
		return plaintext, nil
	}
	key, err := loadOrCreateEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to initialize cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to initialize GCM: %w", err)
	}
	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, sealed...)
	return encryptedValuePrefix + base64.RawURLEncoding.EncodeToString(payload) + encryptedValueSuffix, nil
}

func decryptValue(value string) (string, error) {
	if value == "" || !isEncryptedValue(value) {
		return value, nil
	}
	key, err := loadEncryptionKey()
	if err != nil {
		return "", err
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(value, encryptedValuePrefix), encryptedValueSuffix)
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted value: %w", err)
	}
	if len(payload) <= gcmNonceSize {
		return "", fmt.Errorf("encrypted value payload is too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to initialize cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to initialize GCM: %w", err)
	}
	nonce := payload[:gcmNonceSize]
	ciphertext := payload[gcmNonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt value: %w", err)
	}
	return string(plaintext), nil
}

func isEncryptedValue(value string) bool {
	return strings.HasPrefix(value, encryptedValuePrefix) && strings.HasSuffix(value, encryptedValueSuffix)
}

func loadOrCreateEncryptionKey() ([]byte, error) {
	keyPath, err := getEncryptionKeyPath()
	if err != nil {
		return nil, err
	}
	if key, err := readEncryptionKey(keyPath); err == nil {
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := EnsureConfigDir(); err != nil {
		return nil, err
	}
	key := make([]byte, encryptionKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to write encryption key: %w", err)
	}
	return key, nil
}

func loadEncryptionKey() ([]byte, error) {
	keyPath, err := getEncryptionKeyPath()
	if err != nil {
		return nil, err
	}
	key, err := readEncryptionKey(keyPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("encryption key not found: %s", keyPath)
	}
	return key, err
}

func readEncryptionKey(keyPath string) ([]byte, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	if len(data) != encryptionKeySize {
		return nil, fmt.Errorf("invalid encryption key size in %s", keyPath)
	}
	return data, nil
}

func getEncryptionKeyPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, encryptionKeyFile), nil
}
