package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// BotWallet represents a per-bot wallet with AI coin and USD balances.
type BotWallet struct {
	BotID          string  `json:"bot_id"`
	Address        string  `json:"address"`
	AiCoinBalance  float64 `json:"ai_coin_balance"`
	UsdBalanceCents int64  `json:"usd_balance_cents"`
	ChainID        int     `json:"chain_id"`
	Enabled        bool    `json:"enabled"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// WalletTransaction represents a single wallet transaction event.
type WalletTransaction struct {
	ID             int64   `json:"id"`
	BotID          string  `json:"bot_id"`
	Type           string  `json:"type"`            // fund, withdraw, purchase, refund, earning
	AmountAiCoin   float64 `json:"amount_ai_coin"`
	AmountUsdCents int64   `json:"amount_usd_cents"`
	Source         string  `json:"source"`
	Status         string  `json:"status"`           // pending, confirmed, failed
	ReferenceID    string  `json:"reference_id"`
	Description    string  `json:"description"`
	TxHash         string  `json:"tx_hash"`
	CreatedAt      string  `json:"created_at"`
	ConfirmedAt    string  `json:"confirmed_at,omitempty"`
}

// AutoPurchaseRule represents an automatic capacity purchase rule for a bot.
type AutoPurchaseRule struct {
	ID                string `json:"id"`
	BotID             string `json:"bot_id"`
	CapacityType      string `json:"capacity_type"`
	PreferredProvider string `json:"preferred_provider"`
	PreferredModel    string `json:"preferred_model"`
	MaxCentsPerUnit   int64  `json:"max_cents_per_unit"`
	DefaultQuantity   int    `json:"default_quantity"`
	Enabled           bool   `json:"enabled"`
	MinBalanceTrigger int64  `json:"min_balance_trigger"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// GetBotWallet returns the wallet for a bot, or nil if none exists.
func (ls *LocalStorage) GetBotWallet(ctx context.Context, botID string) (*BotWallet, error) {
	db := ls.requireSQLDB()

	row := db.QueryRowContext(ctx, `
		SELECT bot_id, address, ai_coin_balance, usd_balance_cents,
			chain_id, enabled, created_at, updated_at
		FROM bot_wallets
		WHERE bot_id = ?`, botID)

	var w BotWallet
	if err := row.Scan(
		&w.BotID, &w.Address, &w.AiCoinBalance, &w.UsdBalanceCents,
		&w.ChainID, &w.Enabled, &w.CreatedAt, &w.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows || strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, fmt.Errorf("get bot wallet: %w", err)
	}

	return &w, nil
}

// CreateOrUpdateBotWallet upserts a wallet for a bot.
func (ls *LocalStorage) CreateOrUpdateBotWallet(ctx context.Context, wallet *BotWallet) error {
	if wallet == nil {
		return fmt.Errorf("bot wallet is nil")
	}
	if wallet.BotID == "" {
		return fmt.Errorf("bot_id is required")
	}

	db := ls.requireSQLDB()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.ExecContext(ctx, `
		INSERT INTO bot_wallets (
			bot_id, address, ai_coin_balance, usd_balance_cents,
			chain_id, enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bot_id) DO UPDATE SET
			address = excluded.address,
			ai_coin_balance = excluded.ai_coin_balance,
			usd_balance_cents = excluded.usd_balance_cents,
			chain_id = excluded.chain_id,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at`,
		wallet.BotID, wallet.Address, wallet.AiCoinBalance, wallet.UsdBalanceCents,
		wallet.ChainID, wallet.Enabled, now, now)
	if err != nil {
		return fmt.Errorf("create or update bot wallet: %w", err)
	}

	return nil
}

// FundBotWallet adds funds to a bot wallet and creates a transaction record.
func (ls *LocalStorage) FundBotWallet(ctx context.Context, botID string, amountAiCoin float64, amountUsdCents int64, source, description string) (*WalletTransaction, error) {
	db := ls.requireSQLDB()
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Update wallet balances
	result, err := tx.ExecContext(ctx, `
		UPDATE bot_wallets SET
			ai_coin_balance = ai_coin_balance + ?,
			usd_balance_cents = usd_balance_cents + ?,
			updated_at = ?
		WHERE bot_id = ?`,
		amountAiCoin, amountUsdCents, now, botID)
	if err != nil {
		return nil, fmt.Errorf("update wallet balance: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return nil, fmt.Errorf("wallet not found for bot %s", botID)
	}

	// Create transaction record
	txID := fmt.Sprintf("wal-tx-%d", time.Now().UnixNano())
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_transactions (
			reference_id, bot_id, type, amount_ai_coin, amount_usd_cents,
			source, status, description, tx_hash, created_at, confirmed_at
		) VALUES (?, ?, 'fund', ?, ?, ?, 'confirmed', ?, '', ?, ?)`,
		txID, botID, amountAiCoin, amountUsdCents, source, description, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert wallet transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &WalletTransaction{
		BotID:          botID,
		Type:           "fund",
		AmountAiCoin:   amountAiCoin,
		AmountUsdCents: amountUsdCents,
		Source:         source,
		Status:         "confirmed",
		ReferenceID:    txID,
		Description:    description,
		CreatedAt:      now,
		ConfirmedAt:    now,
	}, nil
}

// WithdrawFromBotWallet withdraws funds from a bot wallet and creates a transaction record.
func (ls *LocalStorage) WithdrawFromBotWallet(ctx context.Context, botID string, amountAiCoin float64, amountUsdCents int64, description string) (*WalletTransaction, error) {
	db := ls.requireSQLDB()
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Check sufficient balance
	var currentAiCoin float64
	var currentUsdCents int64
	err = tx.QueryRowContext(ctx, `
		SELECT ai_coin_balance, usd_balance_cents FROM bot_wallets WHERE bot_id = ?`, botID).
		Scan(&currentAiCoin, &currentUsdCents)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("wallet not found for bot %s", botID)
		}
		return nil, fmt.Errorf("query wallet balance: %w", err)
	}

	if currentAiCoin < amountAiCoin {
		return nil, fmt.Errorf("insufficient AI coin balance: have %.6f, need %.6f", currentAiCoin, amountAiCoin)
	}
	if currentUsdCents < amountUsdCents {
		return nil, fmt.Errorf("insufficient USD balance: have %d cents, need %d cents", currentUsdCents, amountUsdCents)
	}

	// Update wallet balances
	_, err = tx.ExecContext(ctx, `
		UPDATE bot_wallets SET
			ai_coin_balance = ai_coin_balance - ?,
			usd_balance_cents = usd_balance_cents - ?,
			updated_at = ?
		WHERE bot_id = ?`,
		amountAiCoin, amountUsdCents, now, botID)
	if err != nil {
		return nil, fmt.Errorf("update wallet balance: %w", err)
	}

	// Create transaction record
	txID := fmt.Sprintf("wal-tx-%d", time.Now().UnixNano())
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_transactions (
			reference_id, bot_id, type, amount_ai_coin, amount_usd_cents,
			source, status, description, tx_hash, created_at, confirmed_at
		) VALUES (?, ?, 'withdraw', ?, ?, '', 'confirmed', ?, '', ?, ?)`,
		txID, botID, amountAiCoin, amountUsdCents, description, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert wallet transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &WalletTransaction{
		BotID:          botID,
		Type:           "withdraw",
		AmountAiCoin:   amountAiCoin,
		AmountUsdCents: amountUsdCents,
		Status:         "confirmed",
		ReferenceID:    txID,
		Description:    description,
		CreatedAt:      now,
		ConfirmedAt:    now,
	}, nil
}

// GetWalletTransactions returns recent transactions for a bot wallet.
func (ls *LocalStorage) GetWalletTransactions(ctx context.Context, botID string, limit int) ([]*WalletTransaction, error) {
	db := ls.requireSQLDB()

	if limit <= 0 {
		limit = 50
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, bot_id, type, amount_ai_coin, amount_usd_cents,
			source, status, reference_id, description, tx_hash,
			created_at, COALESCE(confirmed_at, '')
		FROM wallet_transactions
		WHERE bot_id = ?
		ORDER BY created_at DESC
		LIMIT ?`,
		botID, limit)
	if err != nil {
		// Table may not exist yet on older schemas — return empty
		if strings.Contains(err.Error(), "no such table") {
			return []*WalletTransaction{}, nil
		}
		return nil, fmt.Errorf("get wallet transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*WalletTransaction
	for rows.Next() {
		var t WalletTransaction
		if err := rows.Scan(
			&t.ID, &t.BotID, &t.Type, &t.AmountAiCoin, &t.AmountUsdCents,
			&t.Source, &t.Status, &t.ReferenceID, &t.Description, &t.TxHash,
			&t.CreatedAt, &t.ConfirmedAt,
		); err != nil {
			return nil, fmt.Errorf("scan wallet transaction: %w", err)
		}
		transactions = append(transactions, &t)
	}

	return transactions, nil
}

// GetAutoPurchaseRules returns all auto-purchase rules for a bot.
func (ls *LocalStorage) GetAutoPurchaseRules(ctx context.Context, botID string) ([]*AutoPurchaseRule, error) {
	db := ls.requireSQLDB()

	rows, err := db.QueryContext(ctx, `
		SELECT id, bot_id, capacity_type, preferred_provider, preferred_model,
			max_cents_per_unit, default_quantity, enabled, min_balance_trigger,
			created_at, updated_at
		FROM auto_purchase_rules
		WHERE bot_id = ?
		ORDER BY created_at DESC`, botID)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []*AutoPurchaseRule{}, nil
		}
		return nil, fmt.Errorf("get auto purchase rules: %w", err)
	}
	defer rows.Close()

	var rules []*AutoPurchaseRule
	for rows.Next() {
		var r AutoPurchaseRule
		if err := rows.Scan(
			&r.ID, &r.BotID, &r.CapacityType, &r.PreferredProvider, &r.PreferredModel,
			&r.MaxCentsPerUnit, &r.DefaultQuantity, &r.Enabled, &r.MinBalanceTrigger,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan auto purchase rule: %w", err)
		}
		rules = append(rules, &r)
	}

	return rules, nil
}

// SaveAutoPurchaseRule upserts an auto-purchase rule.
func (ls *LocalStorage) SaveAutoPurchaseRule(ctx context.Context, rule *AutoPurchaseRule) error {
	if rule == nil {
		return fmt.Errorf("auto purchase rule is nil")
	}
	if rule.BotID == "" {
		return fmt.Errorf("bot_id is required")
	}

	db := ls.requireSQLDB()
	now := time.Now().UTC().Format(time.RFC3339)

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("apr-%d", time.Now().UnixNano())
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO auto_purchase_rules (
			id, bot_id, capacity_type, preferred_provider, preferred_model,
			max_cents_per_unit, default_quantity, enabled, min_balance_trigger,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			capacity_type = excluded.capacity_type,
			preferred_provider = excluded.preferred_provider,
			preferred_model = excluded.preferred_model,
			max_cents_per_unit = excluded.max_cents_per_unit,
			default_quantity = excluded.default_quantity,
			enabled = excluded.enabled,
			min_balance_trigger = excluded.min_balance_trigger,
			updated_at = excluded.updated_at`,
		rule.ID, rule.BotID, rule.CapacityType, rule.PreferredProvider, rule.PreferredModel,
		rule.MaxCentsPerUnit, rule.DefaultQuantity, rule.Enabled, rule.MinBalanceTrigger,
		now, now)
	if err != nil {
		return fmt.Errorf("save auto purchase rule: %w", err)
	}

	return nil
}

// DeleteAutoPurchaseRule removes an auto-purchase rule.
func (ls *LocalStorage) DeleteAutoPurchaseRule(ctx context.Context, botID, ruleID string) error {
	db := ls.requireSQLDB()
	_, err := db.ExecContext(ctx, `DELETE FROM auto_purchase_rules WHERE bot_id = ? AND id = ?`, botID, ruleID)
	if err != nil {
		return fmt.Errorf("delete auto purchase rule: %w", err)
	}
	return nil
}

// GetWalletsSummary returns aggregate wallet statistics across all bots.
func (ls *LocalStorage) GetWalletsSummary(ctx context.Context) (totalBots int, totalAiCoin float64, totalUsdCents int64, err error) {
	db := ls.requireSQLDB()

	row := db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(ai_coin_balance), 0), COALESCE(SUM(usd_balance_cents), 0)
		FROM bot_wallets`)

	if err = row.Scan(&totalBots, &totalAiCoin, &totalUsdCents); err != nil {
		err = fmt.Errorf("get wallets summary: %w", err)
		return
	}

	return
}
