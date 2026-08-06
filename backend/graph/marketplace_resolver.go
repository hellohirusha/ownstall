package graph

// Helpers shared by the marketplace resolvers.
//
// The resolver methods themselves live in marketplace.resolvers.go because
// gqlgen owns that file and copies implementations into it on every generate.
// Anything defined here is left alone by the generator.

import (
	"context"
	"errors"
	"fmt"

	pgx "github.com/jackc/pgx/v5"

	"github.com/hellohirusha/ownstall/graph/model"
	appMiddleware "github.com/hellohirusha/ownstall/internal/middleware"
)

// The column list every Store scan expects, in order. Kept in one place so a
// new listing field cannot be added to one query and forgotten in another.
const storeColumns = `
    t.id, t.name, t.subdomain, t.tagline, t.description, t.category,
    t.location, t.logo_url, t.status, t.is_active,
    t.can_publish_products, t.can_accept_orders,
    (SELECT COUNT(*) FROM products p
      WHERE p.tenant_id = t.id AND p.status = 'active'),
    t.created_at, t.submitted_at, t.reviewed_at, t.review_note`

type storeScanner interface {
	Scan(dest ...any) error
}

func scanStore(row storeScanner) (*model.Store, error) {
	var s model.Store
	var id string
	var productCount int32

	err := row.Scan(
		&id, &s.Name, &s.Subdomain, &s.Tagline, &s.Description, &s.Category,
		&s.Location, &s.LogoURL, &s.Status, &s.IsActive,
		&s.CanPublishProducts, &s.CanAcceptOrders,
		&productCount,
		&s.CreatedAt, &s.SubmittedAt, &s.ReviewedAt, &s.ReviewNote,
	)
	if err != nil {
		return nil, err
	}

	s.ID = parseUUID(id)
	s.ProductCount = productCount
	return &s, nil
}

// requireAdmin gates a resolver on a platform-operator token. The token's
// scope was already checked when it was decoded; this only asserts that the
// caller presented one at all.
func requireAdmin(ctx context.Context) (string, error) {
	adminID := appMiddleware.GetAdminID(ctx)
	if adminID == "" {
		return "", fmt.Errorf("platform administrator access required")
	}
	return adminID, nil
}

func requireTenant(ctx context.Context) (string, error) {
	tenantID := appMiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return "", fmt.Errorf("unauthorized")
	}
	return tenantID, nil
}

func requireBuyer(ctx context.Context) (string, error) {
	buyerID := appMiddleware.GetBuyerID(ctx)
	if buyerID == "" {
		return "", fmt.Errorf("sign in to view your orders")
	}
	return buyerID, nil
}

// storeIsPublic reports whether a stall is visible to shoppers. Used to gate
// the public catalogue queries, which would otherwise happily serve an
// unapproved stall's products to anyone holding its tenant id.
func (r *Resolver) storeIsPublic(ctx context.Context, tenantID string) (bool, error) {
	var status string
	var isActive bool
	err := r.DB.QueryRow(ctx,
		`SELECT status, is_active FROM tenants WHERE id = $1`, tenantID,
	).Scan(&status, &isActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return status == "approved" && isActive, nil
}

// fetchStore reads one stall by an arbitrary predicate.
func (r *Resolver) fetchStore(ctx context.Context, where string, args ...any) (*model.Store, error) {
	query := fmt.Sprintf("SELECT %s FROM tenants t WHERE %s", storeColumns, where)
	return scanStore(r.DB.QueryRow(ctx, query, args...))
}

// recordModeration appends to the operator audit trail. A moderation action
// that is not recorded is one nobody can explain later, so a failure here
// fails the whole mutation rather than being logged and swallowed.
func (r *Resolver) recordModeration(
	ctx context.Context, tenantID, adminID, action string, reason *string,
) error {
	var adminArg any
	if adminID != "" {
		adminArg = adminID
	}

	_, err := r.DB.Exec(ctx, `
        INSERT INTO tenant_moderation_events (tenant_id, admin_id, action, reason)
        VALUES ($1, $2, $3, $4)
    `, tenantID, adminArg, action, reason)
	return err
}

// moderate applies a status change plus its audit row in one transaction, so a
// stall can never end up suspended with no record of who did it or why.
func (r *Resolver) moderate(
	ctx context.Context, tenantIDStr, action, newStatus string, reason *string,
) (*model.Store, error) {
	adminID, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
        UPDATE tenants
           SET status = $2,
               reviewed_at = NOW(),
               reviewed_by = $3,
               review_note = COALESCE($4, review_note),
               -- Restoring a stall clears the restrictions that came with the
               -- suspension; leaving them set would silently keep it crippled.
               can_publish_products = CASE WHEN $2 = 'approved' THEN true ELSE can_publish_products END,
               can_accept_orders    = CASE WHEN $2 = 'approved' THEN true ELSE can_accept_orders END
         WHERE id = $1
    `, tenantIDStr, newStatus, adminID, reason)
	if err != nil {
		return nil, fmt.Errorf("failed to update stall: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("stall not found")
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO tenant_moderation_events (tenant_id, admin_id, action, reason)
        VALUES ($1, $2, $3, $4)
    `, tenantIDStr, adminID, action, reason); err != nil {
		return nil, fmt.Errorf("failed to record moderation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.fetchStore(ctx, "t.id = $1", tenantIDStr)
}

func derefOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}
