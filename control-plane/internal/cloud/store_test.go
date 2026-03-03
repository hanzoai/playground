package cloud

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *NodeStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewNodeStore(filepath.Join(dir, "nodes.db"))
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func sampleNode(id string) *CloudNode {
	return &CloudNode{
		NodeID:         id,
		PodName:        "agent-" + id,
		Namespace:      "hanzo",
		NodeType:       NodeTypeCloud,
		Status:         "Running",
		Image:          "ghcr.io/hanzoai/bot:latest",
		Endpoint:       "ws://gw.hanzo.bot",
		Owner:          "user-1",
		Org:            "hanzo",
		OS:             "linux",
		RemoteProtocol: "vnc",
		Labels: map[string]string{
			"playground.hanzo.ai/node-id": id,
			"playground.hanzo.ai/type":    "cloud",
		},
		CreatedAt: time.Now().Truncate(time.Millisecond),
		LastSeen:  time.Now().Truncate(time.Millisecond),
	}
}

func TestNodeStore_PutAndGet(t *testing.T) {
	store := newTestStore(t)
	node := sampleNode("node-1")

	require.NoError(t, store.Put(node))

	got, err := store.Get("node-1")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, node.NodeID, got.NodeID)
	assert.Equal(t, node.PodName, got.PodName)
	assert.Equal(t, node.Namespace, got.Namespace)
	assert.Equal(t, node.NodeType, got.NodeType)
	assert.Equal(t, node.Status, got.Status)
	assert.Equal(t, node.Image, got.Image)
	assert.Equal(t, node.Owner, got.Owner)
	assert.Equal(t, node.Org, got.Org)
	assert.Equal(t, node.OS, got.OS)
	assert.Equal(t, node.Labels, got.Labels)
}

func TestNodeStore_GetNotFound(t *testing.T) {
	store := newTestStore(t)

	got, err := store.Get("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestNodeStore_List(t *testing.T) {
	store := newTestStore(t)

	require.NoError(t, store.Put(sampleNode("a")))
	require.NoError(t, store.Put(sampleNode("b")))
	require.NoError(t, store.Put(sampleNode("c")))

	nodes, err := store.List()
	require.NoError(t, err)
	assert.Len(t, nodes, 3)

	ids := make(map[string]bool)
	for _, n := range nodes {
		ids[n.NodeID] = true
	}
	assert.True(t, ids["a"])
	assert.True(t, ids["b"])
	assert.True(t, ids["c"])
}

func TestNodeStore_ListEmpty(t *testing.T) {
	store := newTestStore(t)

	nodes, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestNodeStore_Delete(t *testing.T) {
	store := newTestStore(t)

	require.NoError(t, store.Put(sampleNode("del-me")))

	got, err := store.Get("del-me")
	require.NoError(t, err)
	require.NotNil(t, got)

	require.NoError(t, store.Delete("del-me"))

	got, err = store.Get("del-me")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestNodeStore_DeleteNonexistent(t *testing.T) {
	store := newTestStore(t)
	// Deleting a key that does not exist is a no-op.
	require.NoError(t, store.Delete("ghost"))
}

func TestNodeStore_PutOverwrite(t *testing.T) {
	store := newTestStore(t)

	node := sampleNode("overwrite")
	node.Status = "Pending"
	require.NoError(t, store.Put(node))

	node.Status = "Running"
	require.NoError(t, store.Put(node))

	got, err := store.Get("overwrite")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Running", got.Status)
}

func TestNodeStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nodes.db")

	// Write data with one store instance.
	store1, err := NewNodeStore(dbPath)
	require.NoError(t, err)
	require.NoError(t, store1.Put(sampleNode("persistent")))
	require.NoError(t, store1.Close())

	// Open a second store on the same file and verify data survived.
	store2, err := NewNodeStore(dbPath)
	require.NoError(t, err)
	defer store2.Close()

	got, err := store2.Get("persistent")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "persistent", got.NodeID)
}

func TestNodeStore_BillingFields(t *testing.T) {
	store := newTestStore(t)

	node := sampleNode("billing")
	node.HoldID = "hold-abc"
	node.CentsPerHour = 50
	node.BillingUserID = "hanzo/user-1"
	node.ProvisionedAt = time.Now().Truncate(time.Millisecond)

	require.NoError(t, store.Put(node))

	got, err := store.Get("billing")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "hold-abc", got.HoldID)
	assert.Equal(t, 50, got.CentsPerHour)
	assert.Equal(t, "hanzo/user-1", got.BillingUserID)
}

func TestNodeStore_CreatesDirIfMissing(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "deep", "nested", "path")

	store, err := NewNodeStore(filepath.Join(nested, "nodes.db"))
	require.NoError(t, err)
	defer store.Close()

	// Verify the directory was created.
	_, err = os.Stat(nested)
	require.NoError(t, err)
}
