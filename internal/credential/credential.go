// Package credential handles secure API key storage (SPEC §3.1, §3.8).
package credential

import "fmt"

// Manager defines the interface for secure credential operations.
type Manager interface {
	Store(key string) error
	Retrieve() (string, error)
	Delete() error
	Status() string // human-readable status, never includes plaintext key
}

// NewManager returns the best available Manager for the current platform.
func NewManager() Manager {
	return newWincredManager()
}

// MaskKey returns a human-safe representation of a key: last 4 chars only.
func MaskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return fmt.Sprintf("sk-...%s", key[len(key)-4:])
}
