package registry

import (
	"context"

	"github.com/todd2982/watchtower/pkg/registry/helpers"
	watchtowerTypes "github.com/todd2982/watchtower/pkg/types"
	ref "github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
	log "github.com/sirupsen/logrus"
)

// GetPullOptions creates a struct with all options needed for pulling images from a registry.
//
// SECURITY NOTE: This function handles registry authentication credentials.
// Important security considerations:
//
//   - Credentials are read from Docker config files (~/.docker/config.json or system config)
//   - Authentication data is base64-encoded and passed to the Docker daemon
//   - WARNING: Credentials can appear in logs if TRACE level logging is enabled
//     (see commented line 26 - kept commented for security)
//   - Credentials are only used for image pulls and are not stored by watchtower
//   - The Docker daemon handles credential storage and retrieval
//
// Best practices:
//   - Use registry access tokens instead of passwords when possible
//   - Avoid TRACE level logging in production to prevent credential leakage
//   - Ensure Docker config files have appropriate file permissions (0600)
//   - Consider using credential helpers for enhanced security
//
// The function returns empty PullOptions if no authentication is required or configured.
func GetPullOptions(imageName string) (image.PullOptions, error) {
	auth, err := EncodedAuth(imageName)
	log.Debugf("Got image name: %s", imageName)
	if err != nil {
		return image.PullOptions{}, err
	}

	if auth == "" {
		return image.PullOptions{}, nil
	}

	// CREDENTIAL: Uncomment to log docker config auth
	// log.Tracef("Got auth value: %s", auth)

	return image.PullOptions{
		RegistryAuth:  auth,
		PrivilegeFunc: DefaultAuthHandler,
	}, nil
}

// DefaultAuthHandler will be invoked if an AuthConfig is rejected
// It could be used to return a new value for the "X-Registry-Auth" authentication header,
// but there's no point trying again with the same value as used in AuthConfig
func DefaultAuthHandler(ctx context.Context) (string, error) {
	log.Debug("Authentication request was rejected. Trying again without authentication")
	return "", nil
}

// WarnOnAPIConsumption will return true if the registry is known-expected
// to respond well to HTTP HEAD in checking the container digest -- or if there
// are problems parsing the container hostname.
// Will return false if behavior for container is unknown.
func WarnOnAPIConsumption(container watchtowerTypes.Container) bool {

	normalizedRef, err := ref.ParseNormalizedNamed(container.ImageName())
	if err != nil {
		return true
	}

	containerHost, err := helpers.GetRegistryAddress(normalizedRef.Name())
	if err != nil {
		return true
	}

	if containerHost == helpers.DefaultRegistryHost || containerHost == "ghcr.io" {
		return true
	}

	return false
}
