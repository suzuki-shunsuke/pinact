package github

import (
	"context"
	"fmt"
	"os"

	"github.com/1password/onepassword-sdk-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const (
	secretReferenceEnv                = "PINACT_1PASSWORD_GITHUB_TOKEN_SECRET_REFERENCE"
	onepasswordServiceAccountTokenEnv = "PINACT_1PASSWORD_SERVICE_ACCOUNT_TOKEN"
)

type OnePasswordTokenSource struct {
	token           *oauth2.Token
	logE            *logrus.Entry
	secretsAPI      onepassword.SecretsAPI
	secretReference string
}

func get1PasswordSecretReference() string {
	return os.Getenv(secretReferenceEnv)
}

func get1PasswordServiceAccountToken() string {
	return os.Getenv(onepasswordServiceAccountTokenEnv)
}

func new1PasswordTokenSource(ctx context.Context, logE *logrus.Entry) (*OnePasswordTokenSource, error) {
	secretRef := get1PasswordSecretReference()
	saToken := get1PasswordServiceAccountToken()
	if secretRef == "" {
		if saToken != "" {
			logE.Warn(onepasswordServiceAccountTokenEnv + " is not set")
		}
		return nil, nil //nolint:nilnil
	} else if saToken == "" {
		logE.Warn(secretRef + " is not set")
		return nil, nil //nolint:nilnil
	}
	client, err := onepassword.NewClient(
		ctx,
		onepassword.WithServiceAccountToken(saToken),
		onepassword.WithIntegrationInfo("pinact 1Password integration", "v1.0.0"),
	)
	if err != nil {
		return nil, fmt.Errorf("create a 1Password client: %w", err)
	}
	return newOnePasswordTokenSource(logE, client.Secrets(), secretRef), nil
}

func newOnePasswordTokenSource(logE *logrus.Entry, secretsAPI onepassword.SecretsAPI, secretReference string) *OnePasswordTokenSource {
	return &OnePasswordTokenSource{
		logE:            logE,
		secretsAPI:      secretsAPI,
		secretReference: secretReference,
	}
}

func (ks *OnePasswordTokenSource) Token() (*oauth2.Token, error) {
	if ks.token != nil {
		return ks.token, nil
	}
	ks.logE.Debug("getting a GitHub Access toke from 1password")
	s, err := ks.secretsAPI.Resolve(context.Background(), ks.secretReference)
	if err != nil {
		return nil, fmt.Errorf("get a GitHub Access token from 1password: %w", err)
	}
	ks.logE.Debug("got a GitHub Access toke from 1password")
	ks.token = &oauth2.Token{
		AccessToken: s,
	}
	return ks.token, nil
}
