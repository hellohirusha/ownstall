package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedCannedResponses creates default quick-reply templates for a new tenant
func SeedCannedResponses(ctx context.Context, db *pgxpool.Pool, tenantID string) error {
	defaults := []struct {
		Name     string
		Shortcut string
		Body     string
	}{
		{
			Name:     "Order status",
			Shortcut: "/order",
			Body:     "Hi {{.CustomerName}},\n\nThank you for reaching out! I can see your order is currently being processed. You'll receive a shipping notification as soon as it's on its way.\n\nIs there anything else I can help with?",
		},
		{
			Name:     "Refund policy",
			Shortcut: "/refund",
			Body:     "Hi {{.CustomerName}},\n\nWe offer a full refund within 30 days of purchase if you're not completely satisfied. Please reply with your order number and I'll process this right away.\n\nSorry for any inconvenience!",
		},
		{
			Name:     "Shipping time",
			Shortcut: "/shipping",
			Body:     "Hi {{.CustomerName}},\n\nOrders typically ship within 2-3 business days. Once shipped, delivery takes 5-7 business days for standard shipping.\n\nLet me know if you have any other questions!",
		},
		{
			Name:     "Thank you + close",
			Shortcut: "/thanks",
			Body:     "Hi {{.CustomerName}},\n\nThank you for contacting us — I'm glad we could help! I'll go ahead and close this ticket. Feel free to reach out any time if you need anything.\n\nHave a great day!",
		},
	}

	for _, d := range defaults {
		if _, err := db.Exec(ctx, `
            INSERT INTO canned_responses (tenant_id, name, shortcut, body)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (tenant_id, shortcut) DO NOTHING
        `, tenantID, d.Name, d.Shortcut, d.Body); err != nil {
			return fmt.Errorf("failed to seed canned response %s: %w", d.Shortcut, err)
		}
	}
	return nil
}
