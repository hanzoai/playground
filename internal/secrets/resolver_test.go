package secrets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolver_Resolve_PlainValue(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	r := NewResolver(p)

	val, err := r.Resolve(context.Background(), "plain-value")
	require.NoError(t, err)
	require.Equal(t, "plain-value", val)
}

func TestResolver_Resolve_EmptyString(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	r := NewResolver(p)

	val, err := r.Resolve(context.Background(), "")
	require.NoError(t, err)
	require.Empty(t, val)
}

func TestResolver_Resolve_SecretRef(t *testing.T) {
	t.Setenv("PLAYGROUND_MY_API_KEY", "resolved-secret")

	p := NewEnvProvider("PLAYGROUND_")
	r := NewResolver(p)

	val, err := r.Resolve(context.Background(), "secret://MY_API_KEY")
	require.NoError(t, err)
	require.Equal(t, "resolved-secret", val)
}

func TestResolver_Resolve_SecretRef_NotFound(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	r := NewResolver(p)

	val, err := r.Resolve(context.Background(), "secret://TOTALLY_MISSING_KEY_99")
	require.NoError(t, err)
	require.Empty(t, val)
}

func TestResolver_Resolve_EmptySecretRef(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	r := NewResolver(p)

	_, err := r.Resolve(context.Background(), "secret://")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty secret reference")
}

func TestResolver_ResolveMap(t *testing.T) {
	t.Setenv("PLAYGROUND_DB_PASSWORD", "s3cret")

	p := NewEnvProvider("PLAYGROUND_")
	r := NewResolver(p)

	m := map[string]interface{}{
		"host":     "localhost",
		"port":     5432,
		"password": "secret://DB_PASSWORD",
		"nested": map[string]interface{}{
			"token": "secret://DB_PASSWORD",
			"name":  "plain",
		},
	}

	resolved, err := r.ResolveMap(context.Background(), m)
	require.NoError(t, err)

	require.Equal(t, "localhost", resolved["host"])
	require.Equal(t, 5432, resolved["port"])
	require.Equal(t, "s3cret", resolved["password"])

	nested := resolved["nested"].(map[string]interface{})
	require.Equal(t, "s3cret", nested["token"])
	require.Equal(t, "plain", nested["name"])
}

func TestResolver_ResolveMap_Empty(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	r := NewResolver(p)

	resolved, err := r.ResolveMap(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	require.Empty(t, resolved)
}

func TestResolver_Provider(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	r := NewResolver(p)
	require.Equal(t, p, r.Provider())
}
