package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/pkg/queue"
)

type EmailService struct {
	DB    *pgxpool.Pool
	Queue *queue.Client
}

// SeedSystemTemplates creates the default email templates for a new tenant.
// Called when a tenant signs up.
func (s *EmailService) SeedSystemTemplates(ctx context.Context, tenantID string) error {
	templates := []struct {
		Name        string
		Slug        string
		Description string
		Subject     string
		HTMLBody    string
		Variables   []string
	}{
		{
			Name:        "Order Confirmation",
			Slug:        "order_confirmation",
			Description: "Sent automatically when a customer pays",
			Subject:     "Your order #{{.OrderNumber}} is confirmed! 🎉",
			HTMLBody:    orderConfirmationHTML,
			Variables:   []string{"CustomerName", "OrderNumber", "OrderTotal", "OrderItems", "StoreName", "TrackingURL"},
		},
		{
			Name:        "Welcome Email",
			Slug:        "welcome",
			Description: "Sent when a new customer creates an account",
			Subject:     "Welcome to {{.StoreName}}!",
			HTMLBody:    welcomeEmailHTML,
			Variables:   []string{"CustomerName", "StoreName", "StoreURL", "LoginURL"},
		},
		{
			Name:        "Password Reset",
			Slug:        "password_reset",
			Description: "Sent when a user requests a password reset",
			Subject:     "Reset your password for {{.StoreName}}",
			HTMLBody:    passwordResetHTML,
			Variables:   []string{"CustomerName", "StoreName", "ResetURL", "ExpiryTime"},
		},
		{
			Name:        "Shipping Notification",
			Slug:        "shipping_notification",
			Description: "Sent when an order ships",
			Subject:     "Your order is on its way! 📦",
			HTMLBody:    shippingNotificationHTML,
			Variables:   []string{"CustomerName", "OrderNumber", "TrackingNumber", "TrackingURL", "Carrier"},
		},
	}

	for _, t := range templates {
		variablesJSON := fmt.Sprintf(`["%s"]`, strings.Join(t.Variables, ","))
		_, err := s.DB.Exec(ctx, `
            INSERT INTO email_templates
                (tenant_id, name, slug, description, subject, html_body, variables, is_system)
            VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, true)
            ON CONFLICT (tenant_id, slug) DO NOTHING
        `, tenantID, t.Name, t.Slug, t.Description, t.Subject, t.HTMLBody, variablesJSON)
		if err != nil {
			return fmt.Errorf("failed to seed template %s: %w", t.Slug, err)
		}
	}

	return nil
}

// SendTransactional sends a single transactional email immediately
func (s *EmailService) SendTransactional(ctx context.Context,
	tenantID, templateSlug, toEmail, toName string,
	variables map[string]string,
	opts ...TransactionalOption,
) error {
	options := &transactionalOptions{}
	for _, o := range opts {
		o(options)
	}

	// Look up template ID
	var templateID string
	err := s.DB.QueryRow(ctx, `
        SELECT id FROM email_templates
        WHERE tenant_id = $1 AND slug = $2
    `, tenantID, templateSlug).Scan(&templateID)
	if err != nil {
		return fmt.Errorf("template '%s' not found for tenant %s: %w", templateSlug, tenantID, err)
	}

	// Look up template subject for logging
	var subject string
	_ = s.DB.QueryRow(ctx, "SELECT subject FROM email_templates WHERE id = $1", templateID).Scan(&subject)

	// Create email log entry
	var logID string
	if err := s.DB.QueryRow(ctx, `
        INSERT INTO email_logs
            (tenant_id, template_id, to_email, to_name, subject, status, order_id)
        VALUES ($1, $2, $3, $4, $5, 'queued', $6)
        RETURNING id
    `, tenantID, templateID, toEmail, toName, subject, options.orderID).Scan(&logID); err != nil {
		fmt.Printf("WARNING: failed to create email log: %v\n", err)
	}

	// Publish job to queue
	return s.Queue.Publish(ctx, queue.QueueEmail, queue.EmailJobPayload{
		TenantID:   tenantID,
		TemplateID: templateID,
		ToEmail:    toEmail,
		ToName:     toName,
		Variables:  variables,
		OrderID:    options.orderID,
		LogID:      logID,
	})
}

// SendOrderConfirmation is a convenience wrapper for the most common email
func (s *EmailService) SendOrderConfirmation(ctx context.Context, tenantID, orderID string) error {
	// Load order details
	var order struct {
		CustomerEmail string
		CustomerName  string
		Total         float64
		Number        string
	}

	err := s.DB.QueryRow(ctx, `
        SELECT customer_email, COALESCE(customer_name, ''), total,
               '#' || UPPER(SUBSTRING(id::text, 1, 8)) as order_number
        FROM orders WHERE id = $1
    `, orderID).Scan(&order.CustomerEmail, &order.CustomerName, &order.Total, &order.Number)
	if err != nil {
		return fmt.Errorf("order %s not found: %w", orderID, err)
	}

	// Load tenant name for email branding
	var storeName string
	_ = s.DB.QueryRow(ctx, "SELECT name FROM tenants WHERE id = $1", tenantID).Scan(&storeName)

	return s.SendTransactional(ctx, tenantID, "order_confirmation",
		order.CustomerEmail,
		order.CustomerName,
		map[string]string{
			"CustomerName": order.CustomerName,
			"OrderNumber":  order.Number,
			"OrderTotal":   fmt.Sprintf("$%.2f", order.Total),
			"StoreName":    storeName,
		},
		WithOrderID(orderID),
	)
}

// ─────────────────────────────────────────────────────────────
// System email HTML bodies
// ─────────────────────────────────────────────────────────────

const orderConfirmationHTML = `
<h1 style="font-size:24px;font-weight:700;color:#111;margin:0 0 8px;">
    Order confirmed! 🎉
</h1>
<p style="color:#555;margin:0 0 24px;">
    Hi {{.CustomerName}}, thanks for your order from {{.StoreName}}.
</p>

<div style="background:#f9f9f9;border-radius:8px;padding:20px;margin-bottom:24px;">
    <p style="margin:0 0 4px;font-size:13px;color:#999;">ORDER NUMBER</p>
    <p style="margin:0;font-size:20px;font-weight:700;color:#111;font-family:monospace;">
        {{.OrderNumber}}
    </p>
</div>

<table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:24px;">
    <tr>
        <td style="padding:12px 0;border-bottom:1px solid #eee;">
            <p style="margin:0;font-size:13px;color:#999;">ITEMS</p>
        </td>
        <td align="right" style="padding:12px 0;border-bottom:1px solid #eee;">
            <p style="margin:0;font-size:13px;color:#999;">PRICE</p>
        </td>
    </tr>
    {{.OrderItems}}
    <tr>
        <td style="padding:16px 0 0;font-weight:700;color:#111;">Total</td>
        <td align="right" style="padding:16px 0 0;font-weight:700;color:#111;">
            {{.OrderTotal}}
        </td>
    </tr>
</table>

<p style="color:#555;font-size:14px;">
    We will send you a shipping notification when your order is on its way.
</p>
`

const welcomeEmailHTML = `
<h1 style="font-size:24px;font-weight:700;color:#111;margin:0 0 8px;">
    Welcome to {{.StoreName}}! 👋
</h1>
<p style="color:#555;margin:0 0 24px;">
    Hi {{.CustomerName}}, your account is ready. Start exploring our store.
</p>
<a href="{{.StoreURL}}"
   style="display:inline-block;background:#111;color:#fff;padding:12px 24px;
          border-radius:8px;text-decoration:none;font-weight:600;">
    Visit the store
</a>
`

const passwordResetHTML = `
<h1 style="font-size:24px;font-weight:700;color:#111;margin:0 0 8px;">
    Reset your password
</h1>
<p style="color:#555;margin:0 0 24px;">
    Hi {{.CustomerName}}, click the button below to reset your password.
    This link expires in {{.ExpiryTime}}.
</p>
<a href="{{.ResetURL}}"
   style="display:inline-block;background:#111;color:#fff;padding:12px 24px;
          border-radius:8px;text-decoration:none;font-weight:600;">
    Reset password
</a>
<p style="color:#999;font-size:12px;margin-top:24px;">
    If you did not request this, ignore this email. Your password will not change.
</p>
`

const shippingNotificationHTML = `
<h1 style="font-size:24px;font-weight:700;color:#111;margin:0 0 8px;">
    Your order is on its way! 📦
</h1>
<p style="color:#555;margin:0 0 24px;">
    Hi {{.CustomerName}}, order {{.OrderNumber}} has shipped via {{.Carrier}}.
</p>
<div style="background:#f0fdf4;border-radius:8px;padding:20px;margin-bottom:24px;">
    <p style="margin:0 0 4px;font-size:13px;color:#16a34a;">TRACKING NUMBER</p>
    <p style="margin:0;font-size:20px;font-weight:700;color:#111;font-family:monospace;">
        {{.TrackingNumber}}
    </p>
</div>
<a href="{{.TrackingURL}}"
   style="display:inline-block;background:#111;color:#fff;padding:12px 24px;
          border-radius:8px;text-decoration:none;font-weight:600;">
    Track your package
</a>
`

type TransactionalOption func(*transactionalOptions)
type transactionalOptions struct {
	orderID string
}

func WithOrderID(id string) TransactionalOption {
	return func(o *transactionalOptions) { o.orderID = id }
}
