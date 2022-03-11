// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package authorizer

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	environmentVariableAuthorizationURL          = "AUTHORIZATION_URL"
	environmentVariableAuthorizationCABundlePath = "AUTHORIZATION_CA_BUNDLE_PATH"
	environmentVariableKeyPath                   = "KEY_PATH"
	environmentVariableCertificatePath           = "CERTIFICATE_PATH"
)

var (
	errEnvironmentVariableNotFound = errors.New("environment variable not found")
	errFailedToLoadCertificate     = errors.New("failed to load certificate/key")
)

func readEnvironmentVariables() (string, string, string, string, error) {
	authorizationURL, found := os.LookupEnv(environmentVariableAuthorizationURL)
	if !found {
		return "", "", "", "", fmt.Errorf("%w: %s", errEnvironmentVariableNotFound, environmentVariableAuthorizationURL)
	}

	authorizationCABundlePath, found := os.LookupEnv(environmentVariableAuthorizationCABundlePath)
	if !found {
		authorizationCABundlePath = ""
	}

	keyPath, found := os.LookupEnv(environmentVariableKeyPath)
	if !found {
		return "", "", "", "", fmt.Errorf("%w: %s", errEnvironmentVariableNotFound, environmentVariableKeyPath)
	}

	certificatePath, found := os.LookupEnv(environmentVariableCertificatePath)
	if !found {
		return "", "", "", "", fmt.Errorf("%w: %s", errEnvironmentVariableNotFound, environmentVariableCertificatePath)
	}

	return authorizationURL, authorizationCABundlePath, keyPath, certificatePath, nil
}

func readCertificates(authorizationCABundlePath, certificatePath, keyPath string) ([]byte, tls.Certificate, error) {
	var (
		authorizationCABundle []byte
		certificate           tls.Certificate
	)

	if authorizationCABundlePath != "" {
		authorizationCABundle, err := ioutil.ReadFile(authorizationCABundlePath)
		if err != nil {
			return authorizationCABundle, certificate,
				fmt.Errorf("%w: %s", errFailedToLoadCertificate, authorizationCABundle)
		}
	}

	certificate, err := tls.LoadX509KeyPair(certificatePath, keyPath)
	if err != nil {
		return authorizationCABundle, certificate,
			fmt.Errorf("%w: %s/%s", errFailedToLoadCertificate, certificatePath, keyPath)
	}

	return authorizationCABundle, certificate, nil
}

// NewHubOfHubsAuthorizer returns a new instance of HubOfHubsAuthorizer.
func NewHubOfHubsAuthorizer(statusDB db.StatusDB) (*HubOfHubsAuthorizer, error) {
	authorizationURL, authorizationCABundlePath, keyPath, certificatePath, err := readEnvironmentVariables()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize hub of hubs authorizer - %w", err)
	}

	authorizationCABundle, _, err := readCertificates(authorizationCABundlePath, certificatePath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize hub of hubs authorizer - %w", err)
	}

	return &HubOfHubsAuthorizer{
		log:                   ctrl.Log.WithName("hub-of-hubs-authorizer"),
		statusDB:              statusDB,
		authorizationURL:      authorizationURL,
		authorizationCABundle: authorizationCABundle,
	}, nil
}
