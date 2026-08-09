// Package memory handles cross-session memory and context (SPEC §3.7).
package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Entry is a single memory record stored on disk.
type Entry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Store wraps the memory file at the given path.
type Store struct {
	path    string
	Entries []Entry `json:"entries"`
}

// Load reads the memory store from disk or creates an empty one.
func Load(root string) (*Store, error) {
	path := filepath.Join(root, ".gopher", "memory.json")
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	json.Unmarshal(data, &s.Entries)
	return s, nil
}

// Add appends a memory entry and persists.
func (s *Store) Add(entry Entry) error {
	s.Entries = append(s.Entries, entry)
	return s.save()
}

// Clear removes all memory entries.
func (s *Store) Clear() error {
	s.Entries = nil
	return s.save()
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.Entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
