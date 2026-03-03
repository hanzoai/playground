package cloud

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/boltdb/bolt"
)

var nodesBucket = []byte("nodes")

// NodeStore persists CloudNode records in a BoltDB file so node state
// survives process restarts. Without this, a pod restart loses all tracked
// agents, creating orphaned compute and billing leaks.
type NodeStore struct {
	db *bolt.DB
}

// NewNodeStore opens (or creates) a BoltDB file at dbPath and ensures the
// "nodes" bucket exists. The parent directory is created if needed.
func NewNodeStore(dbPath string) (*NodeStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create data directory for node store: %w", err)
	}

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open node store at %s: %w", dbPath, err)
	}

	// Pre-create the bucket so read paths never have to handle a missing bucket.
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(nodesBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize nodes bucket: %w", err)
	}

	return &NodeStore{db: db}, nil
}

// Put serializes a CloudNode to JSON and stores it keyed by NodeID.
func (s *NodeStore) Put(node *CloudNode) error {
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal node %s: %w", node.NodeID, err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(nodesBucket).Put([]byte(node.NodeID), data)
	})
}

// Get retrieves a single CloudNode by ID. Returns nil, nil if not found.
func (s *NodeStore) Get(id string) (*CloudNode, error) {
	var node *CloudNode
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(nodesBucket).Get([]byte(id))
		if v == nil {
			return nil
		}
		node = &CloudNode{}
		return json.Unmarshal(v, node)
	})
	return node, err
}

// List returns all persisted CloudNode records.
func (s *NodeStore) List() ([]*CloudNode, error) {
	var nodes []*CloudNode
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(nodesBucket).ForEach(func(k, v []byte) error {
			var n CloudNode
			if err := json.Unmarshal(v, &n); err != nil {
				return fmt.Errorf("unmarshal node %s: %w", string(k), err)
			}
			nodes = append(nodes, &n)
			return nil
		})
	})
	return nodes, err
}

// Delete removes a node by ID. It is a no-op if the key does not exist.
func (s *NodeStore) Delete(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(nodesBucket).Delete([]byte(id))
	})
}

// Close closes the underlying BoltDB file.
func (s *NodeStore) Close() error {
	return s.db.Close()
}
