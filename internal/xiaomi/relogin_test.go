package xiaomi

import (
	"testing"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/xiaomi"
	"github.com/stretchr/testify/assert"
)

func TestReloginNoTokenNoAccount(t *testing.T) {
	tokens = map[string]string{}
	accounts = map[string]Account{}
	clouds = nil

	_, err := relogin("unknown-user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials available")
}

func TestReloginNoTokenWithBadEncPassword(t *testing.T) {
	tokens = map[string]string{}
	accounts = map[string]Account{
		"user1": {Username: "test", EncPassword: "invalid-base64!!!"},
	}
	clouds = nil

	_, err := relogin("user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot decrypt")
}

func TestRefreshCloudDedup(t *testing.T) {
	// This test verifies the 5-second dedup window.
	// We set up a cloud in the map and a recent lastRefresh,
	// then call refreshCloud — it should return the existing cloud without calling relogin.

	mockCloud := xiaomi.NewCloud(AppXiaomiHome)

	cloudsMu.Lock()
	clouds = map[string]*xiaomi.Cloud{"user1": mockCloud}
	tokens = map[string]string{"user1": "old-token"}
	lastRefresh = map[string]time.Time{"user1": time.Now()}
	cloudsMu.Unlock()

	result, err := refreshCloud("user1")
	assert.NoError(t, err)
	assert.Equal(t, mockCloud, result)
}
