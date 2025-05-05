package github

import (
	"fmt"

	"github.com/zalando/go-keyring"
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
