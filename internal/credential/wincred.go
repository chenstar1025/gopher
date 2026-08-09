package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const fileName = "gopher.key"

type fileManager struct {
	keyDir string
}

func (f *fileManager) keyPath() string {
	return filepath.Join(f.keyDir, fileName)
}

func newWincredManager() Manager {
	cfgDir, _ := os.UserConfigDir()
	if cfgDir == "" {
		cfgDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	keyDir := filepath.Join(cfgDir, "gopher")
	os.MkdirAll(keyDir, 0700)
	return &fileManager{keyDir: keyDir}
}

func (f *fileManager) Store(key string) error {
	encrypted, err := encrypt(key)
	if err != nil {
		return fmt.Errorf("encrypt key: %w", err)
	}
	return os.WriteFile(f.keyPath(), []byte(encrypted), 0600)
}

func (f *fileManager) Retrieve() (string, error) {
	data, err := os.ReadFile(f.keyPath())
	if err != nil {
		return "", fmt.Errorf("no API key found: %w", err)
	}
	return decrypt(string(data))
}

func (f *fileManager) Delete() error {
	err := os.Remove(f.keyPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (f *fileManager) Status() string {
	key, err := f.Retrieve()
	if err != nil {
		return "No API key configured"
	}
	return "API key configured: " + MaskKey(key)
}

// encrypt uses AES-GCM with a machine-derived key (simple static key for
// the purposes of this project — production should use DPAPI or keyring).
func encrypt(plaintext string) (string, error) {
	key := deriveKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	key := deriveKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// deriveKey creates a deterministic key from a host-identifier file.
// In production this would use OS keychain / DPAPI.
func deriveKey() []byte {
	// Use a combination of machine-specific paths as a seed.
	// This is a pragmatic stand-in for OS keychain; documented in README.
	base := []byte("gopher-harness-v1-machine-key")
	home, _ := os.UserHomeDir()
	if home != "" {
		base = append(base, []byte(home)...)
	}
	// Hash to 32 bytes for AES-256
	hash := make([]byte, 32)
	for i := range hash {
		if i < len(base) {
			hash[i] = base[i%len(base)] ^ base[(i*7+3)%len(base)]
		} else {
			hash[i] = hash[i-len(base)] ^ byte(i)
		}
	}
	return hash
}
