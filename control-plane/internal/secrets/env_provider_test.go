package secrets

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvProvider_Type(t *testing.T) {
	p := NewEnvProvider("")
	require.Equal(t, ProviderEnv, p.Type())
}

func TestEnvProvider_GetSecret_EmptyKey(t *testing.T) {
	p := NewEnvProvider("")
	val, err := p.GetSecret(context.Background(), "")
	require.NoError(t, err)
	require.Empty(t, val)
}

func TestEnvProvider_GetSecret_PrefixedEnvVar(t *testing.T) {
	const key = "TEST_KMS_PREF_SECRET"
	const want = "from-playground-env"

	t.Setenv("PLAYGROUND_"+key, want)

	p := NewEnvProvider("PLAYGROUND_")
	val, err := p.GetSecret(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, want, val)
}

func TestEnvProvider_GetSecret_LegacyAgentsFallback(t *testing.T) {
	const key = "TEST_KMS_LEGACY_SECRET"
	const want = "from-agents-env"

	t.Setenv("AGENTS_"+key, want)

	p := NewEnvProvider("PLAYGROUND_")
	val, err := p.GetSecret(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, want, val)
}

func TestEnvProvider_GetSecret_BareKeyFallback(t *testing.T) {
	const key = "TEST_KMS_BARE_SECRET"
	const want = "from-bare-env"

	t.Setenv(key, want)

	p := NewEnvProvider("PLAYGROUND_")
	val, err := p.GetSecret(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, want, val)
}

func TestEnvProvider_GetSecret_PrefixPrecedence(t *testing.T) {
	const key = "TEST_KMS_PREC_SECRET"

	t.Setenv("PLAYGROUND_"+key, "playground-wins")
	t.Setenv("AGENTS_"+key, "agents-loses")
	t.Setenv(key, "bare-loses")

	p := NewEnvProvider("PLAYGROUND_")
	val, err := p.GetSecret(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, "playground-wins", val)
}

func TestEnvProvider_GetSecret_KeyNormalisation(t *testing.T) {
	const envKey = "TEST_KMS_DASH_SECRET"
	const want = "normalised"

	t.Setenv("PLAYGROUND_"+envKey, want)

	p := NewEnvProvider("PLAYGROUND_")

	// Pass dashed variant; should normalise to underscored upper-case.
	val, err := p.GetSecret(context.Background(), "test-kms-dash-secret")
	require.NoError(t, err)
	require.Equal(t, want, val)
}

func TestEnvProvider_GetSecret_NotFound(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	val, err := p.GetSecret(context.Background(), "COMPLETELY_NONEXISTENT_KEY_12345")
	require.NoError(t, err)
	require.Empty(t, val)
}

func TestEnvProvider_SetSecret(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	ctx := context.Background()

	require.NoError(t, p.SetSecret(ctx, "TEST_KMS_SET", "set-value"))

	val, err := p.GetSecret(ctx, "TEST_KMS_SET")
	require.NoError(t, err)
	require.Equal(t, "set-value", val)

	// Should also be in the OS environment.
	require.Equal(t, "set-value", os.Getenv("PLAYGROUND_TEST_KMS_SET"))

	// Cleanup.
	t.Cleanup(func() { _ = os.Unsetenv("PLAYGROUND_TEST_KMS_SET") })
}

func TestEnvProvider_SetSecret_EmptyKey(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	require.NoError(t, p.SetSecret(context.Background(), "", "value"))
}

func TestEnvProvider_DeleteSecret(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	ctx := context.Background()

	// Set first.
	require.NoError(t, p.SetSecret(ctx, "TEST_KMS_DEL", "delete-me"))
	val, err := p.GetSecret(ctx, "TEST_KMS_DEL")
	require.NoError(t, err)
	require.Equal(t, "delete-me", val)

	// Delete.
	require.NoError(t, p.DeleteSecret(ctx, "TEST_KMS_DEL"))

	val, err = p.GetSecret(ctx, "TEST_KMS_DEL")
	require.NoError(t, err)
	require.Empty(t, val)
}

func TestEnvProvider_DeleteSecret_EmptyKey(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	require.NoError(t, p.DeleteSecret(context.Background(), ""))
}

func TestEnvProvider_SetOverridesThenDelete(t *testing.T) {
	p := NewEnvProvider("PLAYGROUND_")
	ctx := context.Background()

	// Override masks the OS env.
	t.Setenv("PLAYGROUND_TEST_KMS_OVRDE", "original")
	require.NoError(t, p.SetSecret(ctx, "TEST_KMS_OVRDE", "overridden"))

	val, err := p.GetSecret(ctx, "TEST_KMS_OVRDE")
	require.NoError(t, err)
	require.Equal(t, "overridden", val)

	// Delete clears override AND env var.
	require.NoError(t, p.DeleteSecret(ctx, "TEST_KMS_OVRDE"))
	val, err = p.GetSecret(ctx, "TEST_KMS_OVRDE")
	require.NoError(t, err)
	require.Empty(t, val)
}

func TestEnvProvider_DefaultPrefix(t *testing.T) {
	p := NewEnvProvider("")
	require.Equal(t, "PLAYGROUND_", p.prefix)
}

func TestNormaliseKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"api-key", "API_KEY"},
		{"api.key", "API_KEY"},
		{"API_KEY", "API_KEY"},
		{"some-mixed.Case_key", "SOME_MIXED_CASE_KEY"},
		{"", ""},
	}

	for _, tc := range tests {
		require.Equal(t, tc.want, normaliseKey(tc.input), "normaliseKey(%q)", tc.input)
	}
}
