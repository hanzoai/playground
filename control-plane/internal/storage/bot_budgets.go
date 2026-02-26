package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// BotBudget represents a per-bot spending budget configuration.
type BotBudget struct {
	BotID           string   `json:"bot_id"`
	MonthlyLimitUSD float64  `json:"monthly_limit_usd"`
	DailyLimitUSD   float64  `json:"daily_limit_usd"`
	AlertThreshold  float64  `json:"alert_threshold"` // 0.0-1.0, triggers warning at this fraction
	Enabled         bool     `json:"enabled"`
	CurrentMonthUSD float64  `json:"current_month_usd"`
	CurrentDayUSD   float64  `json:"current_day_usd"`
	LastResetDate   string   `json:"last_reset_date"`
	UpdatedAt       string   `json:"updated_at"`
	CreatedAt       string   `json:"created_at"`
}

// BotSpendRecord represents a single spend event for a bot.
type BotSpendRecord struct {
	ID          int64   `json:"id"`
	BotID       string  `json:"bot_id"`
	ExecutionID string  `json:"execution_id"`
	AmountUSD   float64 `json:"amount_usd"`
	Description string  `json:"description"`
	RecordedAt  string  `json:"recorded_at"`
}

// BudgetStatus is returned by the budget check to indicate whether an execution is allowed.
type BudgetStatus struct {
	Allowed         bool    `json:"allowed"`
	Reason          string  `json:"reason,omitempty"`
	MonthlyLimitUSD float64 `json:"monthly_limit_usd"`
	MonthlySpentUSD float64 `json:"monthly_spent_usd"`
	DailyLimitUSD   float64 `json:"daily_limit_usd"`
	DailySpentUSD   float64 `json:"daily_spent_usd"`
	AlertTriggered  bool    `json:"alert_triggered"`
}

// GetBotBudget returns the budget configuration for a bot, or nil if none set.
func (ls *LocalStorage) GetBotBudget(ctx context.Context, botID string) (*BotBudget, error) {
	db := ls.requireSQLDB()

	row := db.QueryRowContext(ctx, `
		SELECT bot_id, monthly_limit_usd, daily_limit_usd, alert_threshold,
			enabled, current_month_usd, current_day_usd, last_reset_date,
			updated_at, created_at
		FROM bot_budgets
		WHERE bot_id = ?`, botID)

	var b BotBudget
	if err := row.Scan(
		&b.BotID, &b.MonthlyLimitUSD, &b.DailyLimitUSD, &b.AlertThreshold,
		&b.Enabled, &b.CurrentMonthUSD, &b.CurrentDayUSD, &b.LastResetDate,
		&b.UpdatedAt, &b.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get bot budget: %w", err)
	}

	return &b, nil
}

// SetBotBudget upserts a budget configuration for a bot.
func (ls *LocalStorage) SetBotBudget(ctx context.Context, b *BotBudget) error {
	if b == nil {
		return fmt.Errorf("bot budget is nil")
	}
	if b.BotID == "" {
		return fmt.Errorf("bot_id is required")
	}

	db := ls.requireSQLDB()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.ExecContext(ctx, `
		INSERT INTO bot_budgets (
			bot_id, monthly_limit_usd, daily_limit_usd, alert_threshold,
			enabled, current_month_usd, current_day_usd, last_reset_date,
			updated_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bot_id) DO UPDATE SET
			monthly_limit_usd = excluded.monthly_limit_usd,
			daily_limit_usd = excluded.daily_limit_usd,
			alert_threshold = excluded.alert_threshold,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at`,
		b.BotID, b.MonthlyLimitUSD, b.DailyLimitUSD, b.AlertThreshold,
		b.Enabled, 0.0, 0.0, now, now, now)
	if err != nil {
		return fmt.Errorf("set bot budget: %w", err)
	}

	return nil
}

// DeleteBotBudget removes a budget configuration for a bot.
func (ls *LocalStorage) DeleteBotBudget(ctx context.Context, botID string) error {
	db := ls.requireSQLDB()
	_, err := db.ExecContext(ctx, `DELETE FROM bot_budgets WHERE bot_id = ?`, botID)
	if err != nil {
		return fmt.Errorf("delete bot budget: %w", err)
	}
	return nil
}

// ListBotBudgets returns all budget configurations.
func (ls *LocalStorage) ListBotBudgets(ctx context.Context) ([]*BotBudget, error) {
	db := ls.requireSQLDB()

	rows, err := db.QueryContext(ctx, `
		SELECT bot_id, monthly_limit_usd, daily_limit_usd, alert_threshold,
			enabled, current_month_usd, current_day_usd, last_reset_date,
			updated_at, created_at
		FROM bot_budgets
		ORDER BY bot_id`)
	if err != nil {
		return nil, fmt.Errorf("list bot budgets: %w", err)
	}
	defer rows.Close()

	var budgets []*BotBudget
	for rows.Next() {
		var b BotBudget
		if err := rows.Scan(
			&b.BotID, &b.MonthlyLimitUSD, &b.DailyLimitUSD, &b.AlertThreshold,
			&b.Enabled, &b.CurrentMonthUSD, &b.CurrentDayUSD, &b.LastResetDate,
			&b.UpdatedAt, &b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bot budget: %w", err)
		}
		budgets = append(budgets, &b)
	}

	return budgets, nil
}

// RecordBotSpend adds a spend record and updates the running totals.
func (ls *LocalStorage) RecordBotSpend(ctx context.Context, record *BotSpendRecord) error {
	if record == nil {
		return fmt.Errorf("spend record is nil")
	}

	db := ls.requireSQLDB()
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	currentMonth := now.Format("2006-01")

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Insert spend record
	_, err = tx.ExecContext(ctx, `
		INSERT INTO bot_spend_records (bot_id, execution_id, amount_usd, description, recorded_at)
		VALUES (?, ?, ?, ?, ?)`,
		record.BotID, record.ExecutionID, record.AmountUSD, record.Description, now.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert spend record: %w", err)
	}

	// Reset counters if needed and update totals
	_, err = tx.ExecContext(ctx, `
		UPDATE bot_budgets SET
			current_day_usd = CASE
				WHEN last_reset_date < ? THEN ?
				ELSE current_day_usd + ?
			END,
			current_month_usd = CASE
				WHEN substr(last_reset_date, 1, 7) < ? THEN ?
				ELSE current_month_usd + ?
			END,
			last_reset_date = ?,
			updated_at = ?
		WHERE bot_id = ?`,
		today, record.AmountUSD, record.AmountUSD,
		currentMonth, record.AmountUSD, record.AmountUSD,
		today, now.Format(time.RFC3339), record.BotID)
	if err != nil {
		return fmt.Errorf("update budget totals: %w", err)
	}

	return tx.Commit()
}

// CheckBotBudget verifies whether an execution is allowed within budget constraints.
func (ls *LocalStorage) CheckBotBudget(ctx context.Context, botID string) (*BudgetStatus, error) {
	budget, err := ls.GetBotBudget(ctx, botID)
	if err != nil {
		return nil, err
	}

	// No budget configured = always allowed
	if budget == nil || !budget.Enabled {
		return &BudgetStatus{Allowed: true}, nil
	}

	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	currentMonth := now.Format("2006-01")

	monthlySpent := budget.CurrentMonthUSD
	dailySpent := budget.CurrentDayUSD

	// Reset counters if date rolled over
	if budget.LastResetDate < today {
		dailySpent = 0
	}
	if len(budget.LastResetDate) >= 7 && budget.LastResetDate[:7] < currentMonth {
		monthlySpent = 0
	}

	status := &BudgetStatus{
		Allowed:         true,
		MonthlyLimitUSD: budget.MonthlyLimitUSD,
		MonthlySpentUSD: monthlySpent,
		DailyLimitUSD:   budget.DailyLimitUSD,
		DailySpentUSD:   dailySpent,
	}

	// Check daily limit
	if budget.DailyLimitUSD > 0 && dailySpent >= budget.DailyLimitUSD {
		status.Allowed = false
		status.Reason = fmt.Sprintf("daily budget exceeded: $%.2f / $%.2f", dailySpent, budget.DailyLimitUSD)
	}

	// Check monthly limit
	if budget.MonthlyLimitUSD > 0 && monthlySpent >= budget.MonthlyLimitUSD {
		status.Allowed = false
		status.Reason = fmt.Sprintf("monthly budget exceeded: $%.2f / $%.2f", monthlySpent, budget.MonthlyLimitUSD)
	}

	// Check alert threshold
	if budget.AlertThreshold > 0 && budget.MonthlyLimitUSD > 0 {
		if monthlySpent/budget.MonthlyLimitUSD >= budget.AlertThreshold {
			status.AlertTriggered = true
		}
	}

	return status, nil
}

// GetBotSpendHistory returns spend records for a bot within a time range.
func (ls *LocalStorage) GetBotSpendHistory(ctx context.Context, botID string, since time.Time, limit int) ([]*BotSpendRecord, error) {
	db := ls.requireSQLDB()

	if limit <= 0 {
		limit = 100
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, bot_id, execution_id, amount_usd, description, recorded_at
		FROM bot_spend_records
		WHERE bot_id = ? AND recorded_at >= ?
		ORDER BY recorded_at DESC
		LIMIT ?`,
		botID, since.Format(time.RFC3339), limit)
	if err != nil {
		return nil, fmt.Errorf("get bot spend history: %w", err)
	}
	defer rows.Close()

	var records []*BotSpendRecord
	for rows.Next() {
		var r BotSpendRecord
		if err := rows.Scan(&r.ID, &r.BotID, &r.ExecutionID, &r.AmountUSD, &r.Description, &r.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan spend record: %w", err)
		}
		records = append(records, &r)
	}

	return records, nil
}
