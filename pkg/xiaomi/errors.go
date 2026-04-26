package xiaomi

import (
	"errors"
	"strings"
)

var ErrTokenExpired = errors.New("xiaomi: token expired")

func isTokenExpired(code int, message string) bool {
	if code == 0 {
		return false
	}
	if code == 2 || code == 3 {
		return true
	}
	lower := strings.ToLower(message)
	return strings.Contains(lower, "auth err") ||
		strings.Contains(lower, "invalid signature") ||
		strings.Contains(lower, "servicetoken_expired")
}
