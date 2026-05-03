package gitops

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_GetOrInit(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	r, err := m.GetOrInit("space-1")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "space-1"), r.Path())
	assert.Equal(t, "space-1", r.SpaceID())

	// Second call returns the same cached instance.
	r2, err := m.GetOrInit("space-1")
	require.NoError(t, err)
	assert.Equal(t, r.Path(), r2.Path())
}

func TestManager_MultipleSpaces(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	_, err := m.GetOrInit("s1")
	require.NoError(t, err)
	_, err = m.GetOrInit("s2")
	require.NoError(t, err)
	_, err = m.GetOrInit("s3")
	require.NoError(t, err)

	spaces := m.ListSpaces()
	assert.Len(t, spaces, 3)
}

func TestManager_Remove(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	_, err := m.GetOrInit("s1")
	require.NoError(t, err)

	m.Remove("s1")

	spaces := m.ListSpaces()
	assert.Empty(t, spaces)

	// Can re-init after remove.
	_, err = m.GetOrInit("s1")
	require.NoError(t, err)
	assert.Len(t, m.ListSpaces(), 1)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Hammer GetOrInit from multiple goroutines.
	done := make(chan error, 50)
	for i := 0; i < 50; i++ {
		go func() {
			_, err := m.GetOrInit("shared-space")
			done <- err
		}()
	}

	for i := 0; i < 50; i++ {
		err := <-done
		assert.NoError(t, err)
	}

	assert.Len(t, m.ListSpaces(), 1)
}
