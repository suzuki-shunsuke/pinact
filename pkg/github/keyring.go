package github

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	keyService = "suzuki-shunsuke/pinact"
	keyName    = "GITHUB_TOKEN"
)

type TokenManager struct{}

func NewTokenManager() *TokenManager {
	return &TokenManager{}
}

func (tm *TokenManager) GetToken() (string, error) {
	s, err := keyring.Get(keyService, keyName)
	if err != nil {
		return "", fmt.Errorf("get a GitHub Access token from keyring: %w", err)
	}
	return s, nil
}

func (tm *TokenManager) SetToken(token string) error {
	if err := keyring.Set(keyService, keyName, token); err != nil {
		return fmt.Errorf("set a GitHub Access token in keyring: %w", err)
	}
	return nil
}

func (tm *TokenManager) RemoveToken() error {
	if err := keyring.Delete(keyService, keyName); err != nil {
		return fmt.Errorf("delete a GitHub Access token from keyring: %w", err)
	}
	return nil
}

type KeyringTokenSource struct {
	token *oauth2.Token
	logE  *logrus.Entry
}

func NewKeyringTokenSource(logE *logrus.Entry) *KeyringTokenSource {
	return &KeyringTokenSource{
		logE: logE,
	}
}

func (ks *KeyringTokenSource) Token() (*oauth2.Token, error) {
	if ks.token != nil {
		return ks.token, nil
	}
	ks.logE.Debug("getting a GitHub Access toke from keyring")
	s, err := keyring.Get(keyService, keyName)
	if err != nil {
		return nil, fmt.Errorf("get a GitHub Access token from keyring: %w", err)
	}
	ks.logE.Debug("got a GitHub Access toke from keyring")
	ks.token = &oauth2.Token{
		AccessToken: s,
	}
	return ks.token, nil
}
