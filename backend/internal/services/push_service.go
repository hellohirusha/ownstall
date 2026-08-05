package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/pkg/notifications"
)

// PushService delivers Expo push notifications to a tenant's devices.
type PushService struct {
	DB *pgxpool.Pool
}

// NotifyTenant pushes to every active device registered under a tenant.
// Tokens Expo reports as permanently gone are deactivated so we stop
// paying to retry them.
func (s *PushService) NotifyTenant(ctx context.Context, tenantID, title, body string, data map[string]string) error {
	if s == nil || s.DB == nil {
		return nil
	}

	rows, err := s.DB.Query(ctx, `
        SELECT token FROM device_tokens
        WHERE tenant_id = $1 AND is_active = true
    `, tenantID)
	if err != nil {
		return fmt.Errorf("failed to load device tokens: %w", err)
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return fmt.Errorf("failed to scan device token: %w", err)
		}
		tokens = append(tokens, token)
	}

	if len(tokens) == 0 {
		return nil
	}

	invalid, err := notifications.SendPushNotification(ctx, tokens, title, body, data)

	// Deactivate dead tokens even when the send returned an error —
	// the invalid list is still accurate for the batches that ran
	if len(invalid) > 0 {
		if _, dErr := s.DB.Exec(ctx, `
            UPDATE device_tokens
            SET is_active = false, updated_at = NOW()
            WHERE token = ANY($1)
        `, invalid); dErr != nil {
			fmt.Printf("WARNING: failed to deactivate %d dead push token(s): %v\n", len(invalid), dErr)
		} else {
			fmt.Printf("Deactivated %d unregistered push token(s)\n", len(invalid))
		}
	}

	if err != nil {
		return fmt.Errorf("push send failed: %w", err)
	}
	return nil
}
