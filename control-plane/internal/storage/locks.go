package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hanzoai/playground/control-plane/pkg/types"

	badger "github.com/luxfi/zapdb/v4"
	"github.com/google/uuid"
)

const (
	locksPrefix = "locks:" //nolint:unused // Reserved for future use
)

// AcquireLock attempts to acquire a distributed lock using ZapDB.
func (ls *LocalStorage) AcquireLock(ctx context.Context, key string, timeout time.Duration) (*types.DistributedLock, error) {
	if ls.mode == "postgres" {
		return ls.acquireLockPostgres(ctx, key, timeout)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	lockID := uuid.NewString()
	now := time.Now().UTC()
	expiresAt := now.Add(timeout)
	zapKey := []byte(locksPrefix + key)

	var lock *types.DistributedLock
	err := ls.kvStore.Update(func(txn *badger.Txn) error {
		// Check if lock already exists and is still valid
		item, err := txn.Get(zapKey)
		if err == nil {
			// Key exists — check expiry
			var existing types.DistributedLock
			if valErr := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &existing)
			}); valErr == nil && existing.ExpiresAt.After(now) {
				return fmt.Errorf("lock '%s' is already held by %s", key, existing.Holder)
			}
			// Expired — fall through to overwrite
		} else if err != badger.ErrKeyNotFound {
			return fmt.Errorf("failed to check lock: %w", err)
		}

		lock = &types.DistributedLock{
			LockID:    lockID,
			Key:       key,
			Holder:    lockID,
			ExpiresAt: expiresAt,
			CreatedAt: now,
		}

		data, err := json.Marshal(lock)
		if err != nil {
			return fmt.Errorf("failed to marshal lock: %w", err)
		}

		e := badger.NewEntry(zapKey, data).WithTTL(timeout)
		return txn.SetEntry(e)
	})
	if err != nil {
		return nil, err
	}
	return lock, nil
}

// ReleaseLock releases a distributed lock.
func (ls *LocalStorage) ReleaseLock(ctx context.Context, lockID string) error {
	if ls.mode == "postgres" {
		return ls.releaseLockPostgres(ctx, lockID)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Scan all locks to find one matching this lockID
	return ls.kvStore.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(locksPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(locksPrefix)); it.ValidForPrefix([]byte(locksPrefix)); it.Next() {
			item := it.Item()
			var found bool
			_ = item.Value(func(val []byte) error {
				var lock types.DistributedLock
				if err := json.Unmarshal(val, &lock); err == nil && lock.LockID == lockID {
					found = true
				}
				return nil
			})
			if found {
				return txn.Delete(item.KeyCopy(nil))
			}
		}
		return fmt.Errorf("lock '%s' not found", lockID)
	})
}

// RenewLock renews a distributed lock to extend its TTL.
func (ls *LocalStorage) RenewLock(ctx context.Context, lockID string) (*types.DistributedLock, error) {
	if ls.mode == "postgres" {
		return ls.renewLockPostgres(ctx, lockID)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	newExpiry := time.Now().UTC().Add(30 * time.Second)
	var renewed *types.DistributedLock

	err := ls.kvStore.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(locksPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(locksPrefix)); it.ValidForPrefix([]byte(locksPrefix)); it.Next() {
			item := it.Item()
			var lock types.DistributedLock
			var found bool
			key := item.KeyCopy(nil)
			_ = item.Value(func(val []byte) error {
				if err := json.Unmarshal(val, &lock); err == nil && lock.LockID == lockID {
					found = true
				}
				return nil
			})
			if found {
				it.Close()
				lock.ExpiresAt = newExpiry
				data, err := json.Marshal(&lock)
				if err != nil {
					return err
				}
				ttl := time.Until(newExpiry)
				if ttl <= 0 {
					ttl = time.Second
				}
				e := badger.NewEntry(key, data).WithTTL(ttl)
				renewed = &lock
				return txn.SetEntry(e)
			}
		}
		return fmt.Errorf("lock '%s' not found", lockID)
	})
	if err != nil {
		return nil, err
	}
	return renewed, nil
}

// GetLockStatus retrieves the status of a distributed lock.
func (ls *LocalStorage) GetLockStatus(ctx context.Context, key string) (*types.DistributedLock, error) {
	if ls.mode == "postgres" {
		return ls.getLockStatusPostgres(ctx, key)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var lock *types.DistributedLock
	err := ls.kvStore.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(locksPrefix + key))
		if err == badger.ErrKeyNotFound {
			return nil // no lock held
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			lock = &types.DistributedLock{}
			return json.Unmarshal(val, lock)
		})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get lock status: %w", err)
	}
	return lock, nil
}

func (ls *LocalStorage) acquireLockPostgres(ctx context.Context, key string, timeout time.Duration) (*types.DistributedLock, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	expiresAt := time.Now().UTC().Add(timeout)
	lockID := uuid.NewString()

	query := `
        INSERT INTO distributed_locks(lock_id, key, owner, expires_at, created_at, updated_at)
        VALUES (?, ?, ?, ?, NOW(), NOW())
        ON CONFLICT (key) DO UPDATE SET
                lock_id = EXCLUDED.lock_id,
                owner = EXCLUDED.owner,
                expires_at = EXCLUDED.expires_at,
                updated_at = NOW()
        WHERE distributed_locks.expires_at <= NOW();`

	result, err := ls.db.ExecContext(ctx, query, lockID, key, lockID, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire postgres lock: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to determine lock acquisition result: %w", err)
	}

	if rows == 0 {
		existing, err := ls.getLockStatusPostgres(ctx, key)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ExpiresAt.After(time.Now()) {
			return nil, fmt.Errorf("lock '%s' is already held", key)
		}
		return nil, fmt.Errorf("failed to acquire lock '%s'", key)
	}

	return &types.DistributedLock{
		LockID:    lockID,
		Key:       key,
		Holder:    lockID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (ls *LocalStorage) releaseLockPostgres(ctx context.Context, lockID string) error {
	query := `DELETE FROM distributed_locks WHERE lock_id = ?`
	result, err := ls.db.ExecContext(ctx, query, lockID)
	if err != nil {
		return fmt.Errorf("failed to release postgres lock: %w", err)
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return fmt.Errorf("lock '%s' not found", lockID)
	}
	return nil
}

func (ls *LocalStorage) renewLockPostgres(ctx context.Context, lockID string) (*types.DistributedLock, error) {
	expiresAt := time.Now().UTC().Add(30 * time.Second)
	query := `
        UPDATE distributed_locks
        SET expires_at = ?, updated_at = NOW()
        WHERE lock_id = ?
        RETURNING key, owner, created_at`

	row := ls.db.QueryRowContext(ctx, query, expiresAt, lockID)
	var (
		key       string
		owner     string
		createdAt time.Time
	)
	if err := row.Scan(&key, &owner, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("lock '%s' not found", lockID)
		}
		return nil, fmt.Errorf("failed to renew postgres lock: %w", err)
	}

	return &types.DistributedLock{
		LockID:    lockID,
		Key:       key,
		Holder:    owner,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
	}, nil
}

func (ls *LocalStorage) getLockStatusPostgres(ctx context.Context, key string) (*types.DistributedLock, error) {
	query := `
        SELECT lock_id, owner, expires_at, created_at
        FROM distributed_locks
        WHERE key = ?`

	row := ls.db.QueryRowContext(ctx, query, key)
	var (
		lockID    string
		owner     string
		expiresAt time.Time
		createdAt time.Time
	)

	if err := row.Scan(&lockID, &owner, &expiresAt, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get postgres lock status: %w", err)
	}

	return &types.DistributedLock{
		LockID:    lockID,
		Key:       key,
		Holder:    owner,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
	}, nil
}
