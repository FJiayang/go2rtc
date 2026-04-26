package xiaomi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTokenExpired(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		message string
		want    bool
	}{
		{"code 2", 2, "some message", true},
		{"code 3", 3, "some message", true},
		{"auth err keyword", 99, "auth err", true},
		{"invalid signature keyword", 99, "invalid signature", true},
		{"SERVICETOKEN_EXPIRED keyword", 99, "SERVICETOKEN_EXPIRED", true},
		{"auth err mixed case", 99, "Auth Err in response", true},
		{"normal error", 1, "device not found", false},
		{"code 0 with keyword", 0, "auth err", false},
		{"code 0 normal", 0, "ok", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTokenExpired(tt.code, tt.message)
			assert.Equal(t, tt.want, got)
		})
	}
}
