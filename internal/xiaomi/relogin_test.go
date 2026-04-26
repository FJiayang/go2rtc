package xiaomi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReloginNoTokenNoAccount(t *testing.T) {
	tokens = map[string]string{}
	accounts = map[string]Account{}
	clouds = nil

	_, err := relogin("unknown-user")
	assert.Error(t, err)
}
