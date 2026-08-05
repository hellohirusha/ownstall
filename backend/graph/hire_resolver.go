package graph

import (
	"context"
	"fmt"

	"github.com/hellohirusha/ownstall/graph/model"
)

// creatorProfileColumns is the shared SELECT list for creator profiles.
const creatorProfileColumns = `
    id, display_name, tagline, bio, avatar_url, skills, is_available,
    hourly_rate, response_time, is_published, stripe_onboarded,
    stripe_account_id, total_bookings, completed_bookings,
    avg_rating, total_reviews, created_at`

// fetchCreatorProfile loads one profile matching cond (a WHERE clause over
// creator_profiles columns) along with its services, portfolio and reviews.
// Returns pgx.ErrNoRows-wrapped error when nothing matches.
func (r *Resolver) fetchCreatorProfile(ctx context.Context, cond string, args ...interface{}) (*model.CreatorProfile, error) {
	var p model.CreatorProfile
	var id string
	err := r.DB.QueryRow(ctx,
		"SELECT "+creatorProfileColumns+" FROM creator_profiles WHERE "+cond,
		args...,
	).Scan(
		&id, &p.DisplayName, &p.Tagline, &p.Bio, &p.AvatarURL, &p.Skills,
		&p.IsAvailable, &p.HourlyRate, &p.ResponseTime, &p.IsPublished,
		&p.StripeOnboarded, &p.StripeAccountID, &p.TotalBookings,
		&p.CompletedBookings, &p.AvgRating, &p.TotalReviews, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.ID = parseUUID(id)
	if p.Skills == nil {
		p.Skills = []string{}
	}

	// Services
	p.Services = []*model.CreatorService{}
	svcRows, err := r.DB.Query(ctx, `
        SELECT id, title, description, price, delivery_days, revisions, is_active
        FROM creator_services
        WHERE profile_id = $1
        ORDER BY position, created_at
    `, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query services: %w", err)
	}
	defer svcRows.Close()
	for svcRows.Next() {
		svc, err := scanCreatorService(svcRows)
		if err != nil {
			return nil, err
		}
		p.Services = append(p.Services, svc)
	}

	// Portfolio
	p.PortfolioItems = []*model.PortfolioItem{}
	pfRows, err := r.DB.Query(ctx, `
        SELECT id, title, description, image_url, project_url, tags, position
        FROM portfolio_items
        WHERE profile_id = $1
        ORDER BY position, created_at
    `, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query portfolio: %w", err)
	}
	defer pfRows.Close()
	for pfRows.Next() {
		var item model.PortfolioItem
		var itemID string
		if err := pfRows.Scan(&itemID, &item.Title, &item.Description,
			&item.ImageURL, &item.ProjectURL, &item.Tags, &item.Position); err != nil {
			return nil, fmt.Errorf("failed to scan portfolio item: %w", err)
		}
		item.ID = parseUUID(itemID)
		if item.Tags == nil {
			item.Tags = []string{}
		}
		p.PortfolioItems = append(p.PortfolioItems, &item)
	}

	// Reviews (published only, newest first)
	p.Reviews = []*model.Review{}
	revRows, err := r.DB.Query(ctx, `
        SELECT id, reviewer_name, rating, body, created_at
        FROM reviews
        WHERE profile_id = $1 AND is_published = true
        ORDER BY created_at DESC
    `, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query reviews: %w", err)
	}
	defer revRows.Close()
	for revRows.Next() {
		var rev model.Review
		var revID string
		if err := revRows.Scan(&revID, &rev.ReviewerName, &rev.Rating,
			&rev.Body, &rev.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan review: %w", err)
		}
		rev.ID = parseUUID(revID)
		p.Reviews = append(p.Reviews, &rev)
	}

	return &p, nil
}

func scanCreatorService(row interface{ Scan(dest ...any) error }) (*model.CreatorService, error) {
	var svc model.CreatorService
	var svcID string
	if err := row.Scan(&svcID, &svc.Title, &svc.Description, &svc.Price,
		&svc.DeliveryDays, &svc.Revisions, &svc.IsActive); err != nil {
		return nil, fmt.Errorf("failed to scan service: %w", err)
	}
	svc.ID = parseUUID(svcID)
	return &svc, nil
}

// bookingColumns is the shared SELECT list for bookings.
const bookingColumns = `
    b.id, b.title, b.description, b.status, b.payment_status,
    b.agreed_price, b.platform_fee, b.creator_payout,
    b.client_email, b.client_name, b.delivered_at, b.completed_at, b.created_at`

// scanBooking maps one booking row (selected with bookingColumns) onto the
// generated GraphQL type. Nested service/profile/messages are loaded by the
// Booking field resolvers, so they are correct in both single and list reads.
func scanBooking(row interface{ Scan(dest ...any) error }) (*model.Booking, error) {
	var b model.Booking
	var id string
	if err := row.Scan(
		&id, &b.Title, &b.Description, &b.Status, &b.PaymentStatus,
		&b.AgreedPrice, &b.PlatformFee, &b.CreatorPayout,
		&b.ClientEmail, &b.ClientName, &b.DeliveredAt, &b.CompletedAt, &b.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan booking: %w", err)
	}
	b.ID = parseUUID(id)
	return &b, nil
}

// fetchBooking loads a tenant-owned booking. Its service, profile and
// message thread are filled in by the Booking field resolvers on demand.
func (r *Resolver) fetchBooking(ctx context.Context, tenantID, bookingID string) (*model.Booking, error) {
	row := r.DB.QueryRow(ctx, `
        SELECT `+bookingColumns+`
        FROM bookings b
        WHERE b.id = $1 AND b.tenant_id = $2
    `, bookingID, tenantID)

	return scanBooking(row)
}

// userIdentity returns the email and display name of a user
func (r *Resolver) userIdentity(ctx context.Context, userID string) (string, string, error) {
	var email, name string
	err := r.DB.QueryRow(ctx, `
        SELECT email,
               COALESCE(NULLIF(TRIM(COALESCE(first_name,'') || ' ' || COALESCE(last_name,'')), ''), '')
        FROM users WHERE id = $1
    `, userID).Scan(&email, &name)
	if err != nil {
		return "", "", fmt.Errorf("user not found: %w", err)
	}
	return email, name, nil
}
