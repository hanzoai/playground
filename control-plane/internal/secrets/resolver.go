package secrets

import (
	"context"
	"fmt"
	"strings"
)

// SecretRef is the prefix used in YAML config values to indicate that a value
// should be resolved from the secrets provider.  For example:
//
//	api:
//	  auth:
//	    api_key: "secret://API_KEY"
//
// When the resolver encounters this prefix it strips it and calls
// Provider.GetSecret with the remaining string as the key.
const SecretRef = "secret://"

// Resolver reads config values and resolves any secret:// references using the
// configured Provider. Values that do not begin with the SecretRef prefix are
// returned as-is.
type Resolver struct {
	provider Provider
}

// NewResolver creates a Resolver backed by the given Provider.
func NewResolver(p Provider) *Resolver {
	return &Resolver{provider: p}
}

// Resolve returns the value unchanged if it is not a secret reference,
// or resolves it through the Provider when it starts with "secret://".
func (r *Resolver) Resolve(ctx context.Context, value string) (string, error) {
	if !strings.HasPrefix(value, SecretRef) {
		return value, nil
	}

	key := strings.TrimPrefix(value, SecretRef)
	if key == "" {
		return "", fmt.Errorf("empty secret reference")
	}

	secret, err := r.provider.GetSecret(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to resolve secret %q: %w", key, err)
	}

	return secret, nil
}

// ResolveMap resolves all string values in a map that carry the secret://
// prefix.  Non-string values and non-prefixed strings are left untouched.
func (r *Resolver) ResolveMap(ctx context.Context, m map[string]interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case string:
			resolved, err := r.Resolve(ctx, val)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", k, err)
			}
			out[k] = resolved
		case map[string]interface{}:
			sub, err := r.ResolveMap(ctx, val)
			if err != nil {
				return nil, err
			}
			out[k] = sub
		default:
			out[k] = v
		}
	}
	return out, nil
}

// Provider returns the underlying Provider so callers can read/write secrets
// directly when needed.
func (r *Resolver) Provider() Provider {
	return r.provider
}
