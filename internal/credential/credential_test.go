package credential

import "testing"

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sk-1234567890abcdefgh", "sk-...efgh"},
		{"abc", "****"},
		{"abcd", "****"},
		{"abcde", "sk-...bcde"},
	}

	for _, tt := range tests {
		got := MaskKey(tt.input)
		if got != tt.expected {
			t.Errorf("MaskKey(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMaskKey_NeverLeaksPlaintext(t *testing.T) {
	key := "sk-this-is-a-secret-key-1234"
	masked := MaskKey(key)

	// The full key must not appear in the masked version
	if len(key) > 8 && masked == key {
		t.Error("MaskKey returned the full key!")
	}
	// Only the last 4 chars should be visible (beyond the prefix)
	if len(masked) > 16 {
		t.Errorf("masked key too long: %s (%d chars)", masked, len(masked))
	}
}
