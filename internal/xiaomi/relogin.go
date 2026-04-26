package xiaomi

import (
	"fmt"

	"github.com/AlexxIT/go2rtc/pkg/xiaomi"
)

func relogin(userID string) (*xiaomi.Cloud, error) {
	cloud := xiaomi.NewCloud(AppXiaomiHome)

	token, hasToken := tokens[userID]
	if hasToken {
		if err := cloud.LoginWithToken(userID, token); err == nil {
			return cloud, nil
		}
		log.Debug().Str("user", userID).Msg("[xiaomi] LoginWithToken failed, trying password")
	}

	acct, hasAccount := accounts[userID]
	if !hasAccount || acct.EncPassword == "" {
		if hasToken {
			log.Warn().Str("user", userID).Msg("[xiaomi] no stored password, UI relogin required")
		}
		return nil, fmt.Errorf("xiaomi: no credentials available for user %s", userID)
	}

	password, err := xiaomi.Decrypt(acct.EncPassword)
	if err != nil {
		log.Warn().Err(err).Str("user", userID).Msg("[xiaomi] decrypt password failed, UI relogin required")
		return nil, fmt.Errorf("xiaomi: cannot decrypt password for user %s: %w", userID, err)
	}

	if err = cloud.Login(acct.Username, password); err != nil {
		return nil, err
	}

	return cloud, nil
}
