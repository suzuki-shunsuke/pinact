package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Resolver resolves mutable container image references to immutable digests.
type Resolver interface {
	ResolveDigest(ctx context.Context, image string) (string, error)
}

// DigestResolver resolves container tags via the registry API.
type DigestResolver struct{}

// NewResolver creates a new container digest resolver.
func NewResolver() Resolver {
	return &DigestResolver{}
}

// ResolveDigest resolves image to its current digest.
func (r *DigestResolver) ResolveDigest(ctx context.Context, image string) (string, error) {
	ref, err := name.ParseReference(strings.TrimSpace(strings.TrimPrefix(image, "docker://")))
	if err != nil {
		return "", fmt.Errorf("parse container reference: %w", err)
	}

	resp, err := remote.Head(ref,
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", fmt.Errorf("lookup container reference: %w", err)
	}
	return resp.Digest.String(), nil
}
