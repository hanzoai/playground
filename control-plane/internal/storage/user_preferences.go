package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UserPreferences is the API-facing representation of user notification/voice preferences.
type UserPreferences struct {
	UserID                string  `json:"user_id"`
	NotificationSound     string  `json:"notification_sound"`
	NotificationVolume    float64 `json:"notification_volume"`
	SoundOnTaskComplete   bool    `json:"sound_on_task_complete"`
	SoundOnApprovalNeeded bool    `json:"sound_on_approval_needed"`
	VoiceInputEnabled     bool    `json:"voice_input_enabled"`
	VoiceOutputEnabled    bool    `json:"voice_output_enabled"`
	VoiceOutputVoice      string  `json:"voice_output_voice"`
	OnboardingComplete    bool    `json:"onboarding_complete"`
	UpdatedAt             string  `json:"updated_at"`
}

// DefaultUserPreferences returns sensible defaults for a new user.
func DefaultUserPreferences(userID string) *UserPreferences {
	return &UserPreferences{
		UserID:                userID,
		NotificationSound:     "chime",
		NotificationVolume:    0.7,
		SoundOnTaskComplete:   true,
		SoundOnApprovalNeeded: true,
		VoiceInputEnabled:     false,
		VoiceOutputEnabled:    false,
		VoiceOutputVoice:      "",
		OnboardingComplete:    false,
		UpdatedAt:             time.Now().UTC().Format(time.RFC3339),
	}
}

// GetUserPreferences fetches preferences for a user, returning nil if none exist.
func (ls *LocalStorage) GetUserPreferences(ctx context.Context, userID string) (*UserPreferences, error) {
	db := ls.requireSQLDB()

	row := db.QueryRowContext(ctx, `
		SELECT user_id, notification_sound, notification_volume,
			sound_on_task_complete, sound_on_approval_needed,
			voice_input_enabled, voice_output_enabled, voice_output_voice,
			onboarding_complete, updated_at
		FROM user_preferences
		WHERE user_id = ?`, userID)

	var p UserPreferences
	if err := row.Scan(
		&p.UserID,
		&p.NotificationSound,
		&p.NotificationVolume,
		&p.SoundOnTaskComplete,
		&p.SoundOnApprovalNeeded,
		&p.VoiceInputEnabled,
		&p.VoiceOutputEnabled,
		&p.VoiceOutputVoice,
		&p.OnboardingComplete,
		&p.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user preferences: %w", err)
	}

	return &p, nil
}

// SetUserPreferences upserts preferences for a user.
func (ls *LocalStorage) SetUserPreferences(ctx context.Context, p *UserPreferences) error {
	if p == nil {
		return fmt.Errorf("user preferences is nil")
	}
	if p.UserID == "" {
		return fmt.Errorf("user_id is required")
	}

	db := ls.requireSQLDB()
	now := time.Now().UTC()

	_, err := db.ExecContext(ctx, `
		INSERT INTO user_preferences (
			user_id, notification_sound, notification_volume,
			sound_on_task_complete, sound_on_approval_needed,
			voice_input_enabled, voice_output_enabled, voice_output_voice,
			onboarding_complete, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			notification_sound = excluded.notification_sound,
			notification_volume = excluded.notification_volume,
			sound_on_task_complete = excluded.sound_on_task_complete,
			sound_on_approval_needed = excluded.sound_on_approval_needed,
			voice_input_enabled = excluded.voice_input_enabled,
			voice_output_enabled = excluded.voice_output_enabled,
			voice_output_voice = excluded.voice_output_voice,
			onboarding_complete = excluded.onboarding_complete,
			updated_at = excluded.updated_at`,
		p.UserID, p.NotificationSound, p.NotificationVolume,
		p.SoundOnTaskComplete, p.SoundOnApprovalNeeded,
		p.VoiceInputEnabled, p.VoiceOutputEnabled, p.VoiceOutputVoice,
		p.OnboardingComplete, now)
	if err != nil {
		return fmt.Errorf("set user preferences: %w", err)
	}

	return nil
}
