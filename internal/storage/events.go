package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/playground/pkg/types"

	badger "github.com/luxfi/zapdb/v4"
)

const (
	eventsBucket    = "events"
	defaultEventTTL = 48 * time.Hour // Default TTL for events
)

var (
	cleanupOnce sync.Once
)

// StoreEvent saves a memory change event to the database.
func (ls *LocalStorage) StoreEvent(ctx context.Context, event *types.MemoryChangeEvent) error {
	if ls.mode == "postgres" {
		return ls.storeEventPostgres(ctx, event)
	}

	// Check context cancellation early
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled during store event: %w", err)
	}

	// Start cleanup process once
	cleanupOnce.Do(func() {
		go ls.startEventCleanup()
	})

	return ls.kvStore.Update(func(txn *badger.Txn) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during event transaction: %w", err)
		}

		event.ID = fmt.Sprintf("%d", time.Now().UnixNano())
		event.Timestamp = time.Now().UTC()

		eventJSON, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		key := fmt.Sprintf("events:%s", event.ID)
		return txn.Set([]byte(key), eventJSON)
	})
}

// startEventCleanup starts a background goroutine to clean up expired events.
func (ls *LocalStorage) startEventCleanup() {
	ticker := time.NewTicker(1 * time.Hour) // Clean up every hour
	defer ticker.Stop()

	for range ticker.C {
		if ls.mode == "postgres" {
			ls.cleanupExpiredEventsPostgres()
		} else {
			ls.cleanupExpiredEvents()
		}
	}
}

// cleanupExpiredEvents removes events older than the TTL.
func (ls *LocalStorage) cleanupExpiredEvents() {
	if ls.kvStore == nil {
		return
	}

	cutoff := time.Now().UTC().Add(-defaultEventTTL)

	err := ls.kvStore.Update(func(txn *badger.Txn) error {
		prefix := []byte("events:")
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		var keysToDelete [][]byte
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			_ = item.Value(func(val []byte) error {
				var event types.MemoryChangeEvent
				if err := json.Unmarshal(val, &event); err != nil {
					keysToDelete = append(keysToDelete, append([]byte{}, item.Key()...))
					return nil
				}
				if event.Timestamp.Before(cutoff) {
					keysToDelete = append(keysToDelete, append([]byte{}, item.Key()...))
				}
				return nil
			})
		}
		it.Close()

		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return fmt.Errorf("failed to delete expired event: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error cleaning up expired events: %v\n", err)
	}
}

// GetEventHistory retrieves a list of memory change events based on a filter.
func (ls *LocalStorage) GetEventHistory(ctx context.Context, filter types.EventFilter) ([]*types.MemoryChangeEvent, error) {
	if ls.mode == "postgres" {
		return ls.getEventHistoryPostgres(ctx, filter)
	}

	// Check context cancellation early
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled during get event history: %w", err)
	}

	var events []*types.MemoryChangeEvent
	err := ls.kvStore.View(func(txn *badger.Txn) error {
		prefix := []byte("events:")
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context cancelled during event iteration: %w", err)
			}

			item := it.Item()
			if err := item.Value(func(val []byte) error {
				var event types.MemoryChangeEvent
				if err := json.Unmarshal(val, &event); err != nil {
					return nil // skip corrupted events
				}

				if filter.Scope != nil && event.Scope != *filter.Scope {
					return nil
				}
				if filter.ScopeID != nil && event.ScopeID != *filter.ScopeID {
					return nil
				}
				if filter.Since != nil && event.Timestamp.Before(*filter.Since) {
					return nil
				}

				if len(filter.Patterns) > 0 {
					match := false
					for _, pattern := range filter.Patterns {
						if matched, _ := filepath.Match(pattern, event.Key); matched {
							match = true
							break
						}
					}
					if !match {
						return nil
					}
				}

				events = append(events, &event)
				return nil
			}); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get event history: %w", err)
	}

	// Apply limit after filtering
	if filter.Limit > 0 && len(events) > filter.Limit {
		events = events[len(events)-filter.Limit:]
	}

	return events, nil
}

func (ls *LocalStorage) storeEventPostgres(ctx context.Context, event *types.MemoryChangeEvent) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled during store event: %w", err)
	}

	cleanupOnce.Do(func() {
		go ls.startEventCleanup()
	})

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal event metadata: %w", err)
	}

	event.Timestamp = time.Now().UTC()

	query := `
        INSERT INTO memory_events(scope, scope_id, key, event_type, action, data, previous_data, metadata, timestamp)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        RETURNING id`

	var id sql.NullInt64
	row := ls.db.QueryRowContext(ctx, query,
		event.Scope,
		event.ScopeID,
		event.Key,
		event.Type,
		event.Action,
		event.Data,
		event.PreviousData,
		metadataJSON,
		event.Timestamp,
	)
	if err := row.Scan(&id); err != nil {
		return fmt.Errorf("failed to insert memory event: %w", err)
	}

	if id.Valid {
		event.ID = fmt.Sprintf("%d", id.Int64)
	}

	return nil
}

func (ls *LocalStorage) cleanupExpiredEventsPostgres() {
	cutoff := time.Now().UTC().Add(-defaultEventTTL)
	_, err := ls.db.Exec("DELETE FROM memory_events WHERE timestamp < ?", cutoff)
	if err != nil {
		fmt.Printf("Error cleaning up expired events: %v\n", err)
	}
}

func (ls *LocalStorage) getEventHistoryPostgres(ctx context.Context, filter types.EventFilter) ([]*types.MemoryChangeEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled during get event history: %w", err)
	}

	baseQuery := "SELECT id, scope, scope_id, key, event_type, action, data, previous_data, metadata, timestamp FROM memory_events"
	var conditions []string
	var args []interface{}

	if filter.Scope != nil {
		conditions = append(conditions, "scope = ?")
		args = append(args, *filter.Scope)
	}
	if filter.ScopeID != nil {
		conditions = append(conditions, "scope_id = ?")
		args = append(args, *filter.ScopeID)
	}
	if filter.Since != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *filter.Since)
	}

	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	baseQuery += " ORDER BY timestamp ASC"

	rows, err := ls.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query memory events: %w", err)
	}
	defer rows.Close()

	events := []*types.MemoryChangeEvent{}
	for rows.Next() {
		var (
			id        sql.NullInt64
			scope     string
			scopeID   string
			key       string
			eventType sql.NullString
			action    sql.NullString
			data      []byte
			previous  []byte
			metadata  []byte
			timestamp time.Time
		)
		if err := rows.Scan(&id, &scope, &scopeID, &key, &eventType, &action, &data, &previous, &metadata, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan memory event: %w", err)
		}

		event := &types.MemoryChangeEvent{
			Scope:     scope,
			ScopeID:   scopeID,
			Key:       key,
			Timestamp: timestamp.UTC(),
		}

		if id.Valid {
			event.ID = fmt.Sprintf("%d", id.Int64)
		}

		if eventType.Valid {
			event.Type = eventType.String
		}
		if action.Valid {
			event.Action = action.String
		}
		if len(data) > 0 {
			event.Data = append([]byte(nil), data...)
		}
		if len(previous) > 0 {
			event.PreviousData = append([]byte(nil), previous...)
		}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &event.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal memory event metadata: %w", err)
			}
		}

		if len(filter.Patterns) > 0 {
			match := false
			for _, pattern := range filter.Patterns {
				if matched, _ := filepath.Match(pattern, event.Key); matched {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating memory events: %w", err)
	}

	if filter.Limit > 0 && len(events) > filter.Limit {
		events = events[len(events)-filter.Limit:]
	}

	return events, nil
}
