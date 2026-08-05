package services

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingService struct {
	DB            *pgxpool.Pool
	StripeConnect *StripeConnectService
	EmailService  *EmailService
}

// CreateBooking initiates a booking and creates a PaymentIntent
type CreateBookingInput struct {
	TenantID     string
	ProfileID    string
	ServiceID    string
	ClientEmail  string
	ClientName   string
	Title        string
	Description  string
	Requirements string
	DeliveryDate *time.Time
	AgreedPrice  float64
}

type BookingWithPayment struct {
	BookingID     string
	ClientSecret  string // Stripe PaymentIntent client secret for frontend
	AgreedPrice   float64
	PlatformFee   float64
	CreatorPayout float64
}

func (s *BookingService) CreateBooking(ctx context.Context, input CreateBookingInput) (*BookingWithPayment, error) {
	// Calculate fees
	platformFeePercent := 0.10
	platformFee := input.AgreedPrice * platformFeePercent
	creatorPayout := input.AgreedPrice - platformFee

	// Look up creator's Stripe account — the tenant filter also verifies
	// the profile actually belongs to the tenant being booked against
	var stripeAccountID string
	err := s.DB.QueryRow(ctx, `
        SELECT COALESCE(stripe_account_id, '')
        FROM creator_profiles
        WHERE id = $1 AND tenant_id = $2
    `, input.ProfileID, input.TenantID).Scan(&stripeAccountID)
	if err != nil {
		return nil, fmt.Errorf("creator profile not found: %w", err)
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// Rollback is a no-op after a successful commit
	defer func() { _ = tx.Rollback(ctx) }()

	// Create the booking record
	var bookingID string
	err = tx.QueryRow(ctx, `
        INSERT INTO bookings (
            tenant_id, profile_id, service_id, client_email, client_name,
            title, description, requirements, delivery_date,
            agreed_price, platform_fee, creator_payout, status
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'pending')
        RETURNING id
    `,
		input.TenantID, input.ProfileID, nullableString(input.ServiceID),
		input.ClientEmail, input.ClientName, input.Title,
		input.Description, input.Requirements, input.DeliveryDate,
		input.AgreedPrice, platformFee, creatorPayout,
	).Scan(&bookingID)
	if err != nil {
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	// Keep the denormalized profile stat in step with the new booking
	_, err = tx.Exec(ctx, `
        UPDATE creator_profiles
        SET total_bookings = total_bookings + 1
        WHERE id = $1
    `, input.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to update booking count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Create Stripe PaymentIntent (after DB commit, not inside transaction)
	var clientSecret string
	if stripeAccountID != "" {
		piID, secret, err := s.StripeConnect.CreateBookingPayment(
			ctx, bookingID, input.AgreedPrice,
			stripeAccountID, input.ClientEmail,
		)
		if err != nil {
			// Non-fatal: booking exists, payment can be retried
			fmt.Printf("WARNING: Stripe PaymentIntent failed for booking %s: %v\n", bookingID, err)
		} else {
			clientSecret = secret
			if _, err := s.DB.Exec(ctx,
				"UPDATE bookings SET stripe_payment_intent_id = $1 WHERE id = $2",
				piID, bookingID,
			); err != nil {
				fmt.Printf("WARNING: failed to store payment intent for booking %s: %v\n", bookingID, err)
			}
		}
	} else {
		// Creator hasn't connected Stripe yet — invoice manually
		fmt.Printf("Creator %s has no Stripe account. Booking %s created without payment.\n",
			input.ProfileID, bookingID)
	}

	return &BookingWithPayment{
		BookingID:     bookingID,
		ClientSecret:  clientSecret,
		AgreedPrice:   input.AgreedPrice,
		PlatformFee:   platformFee,
		CreatorPayout: creatorPayout,
	}, nil
}

// AcceptBooking — creator accepts a pending booking
func (s *BookingService) AcceptBooking(ctx context.Context, tenantID, bookingID, creatorUserID string) error {
	result, err := s.DB.Exec(ctx, `
        UPDATE bookings b
        SET status = 'accepted', accepted_at = NOW()
        FROM creator_profiles cp
        WHERE b.id = $1
          AND b.tenant_id = $2
          AND cp.id = b.profile_id
          AND cp.user_id = $3
          AND b.status = 'pending'
    `, bookingID, tenantID, creatorUserID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("booking not found or you are not the creator")
	}

	return nil
}

// DeclineBooking — creator turns down a pending booking
func (s *BookingService) DeclineBooking(ctx context.Context, tenantID, bookingID, creatorUserID string) error {
	result, err := s.DB.Exec(ctx, `
        UPDATE bookings b
        SET status = 'declined', declined_at = NOW()
        FROM creator_profiles cp
        WHERE b.id = $1
          AND b.tenant_id = $2
          AND cp.id = b.profile_id
          AND cp.user_id = $3
          AND b.status = 'pending'
    `, bookingID, tenantID, creatorUserID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("booking not found, not yours, or no longer pending")
	}

	// The booking never went ahead, so it should not count toward the
	// creator's booking total (incremented when the request came in)
	if _, err := s.DB.Exec(ctx, `
        UPDATE creator_profiles cp
        SET total_bookings = GREATEST(cp.total_bookings - 1, 0)
        FROM bookings b
        WHERE b.id = $1 AND cp.id = b.profile_id
    `, bookingID); err != nil {
		fmt.Printf("WARNING: failed to decrement booking count for %s: %v\n", bookingID, err)
	}

	return nil
}

// DeliverBooking — creator marks work as delivered
func (s *BookingService) DeliverBooking(ctx context.Context, tenantID, bookingID, creatorUserID string) error {
	result, err := s.DB.Exec(ctx, `
        UPDATE bookings b
        SET status = 'delivered', delivered_at = NOW()
        FROM creator_profiles cp
        WHERE b.id = $1
          AND b.tenant_id = $2
          AND cp.id = b.profile_id
          AND cp.user_id = $3
          AND b.status IN ('accepted', 'in_progress')
    `, bookingID, tenantID, creatorUserID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("booking not found, not yours, or not in a deliverable state")
	}

	return nil
}

// CompleteBooking — client approves delivery and triggers payout
func (s *BookingService) CompleteBooking(ctx context.Context, tenantID, bookingID, clientEmail string) error {
	var booking struct {
		Status string
		Client string
	}

	err := s.DB.QueryRow(ctx, `
        SELECT status, client_email FROM bookings
        WHERE id = $1 AND tenant_id = $2
    `, bookingID, tenantID).Scan(&booking.Status, &booking.Client)
	if err != nil {
		return fmt.Errorf("booking not found: %w", err)
	}

	if booking.Client != clientEmail {
		return fmt.Errorf("only the client can complete a booking")
	}

	if booking.Status != "delivered" {
		return fmt.Errorf("booking must be delivered before it can be completed")
	}

	// Release escrow to creator. When there's nothing to release (creator
	// not onboarded, no payment) the booking still completes — the payout
	// stays pending and can be retried once Stripe is connected.
	if err := s.StripeConnect.ReleaseEscrow(ctx, bookingID); err != nil {
		fmt.Printf("WARNING: escrow release failed for booking %s: %v\n", bookingID, err)

		if _, err := s.DB.Exec(ctx, `
            UPDATE bookings
            SET status = 'completed', completed_at = NOW()
            WHERE id = $1 AND status = 'delivered'
        `, bookingID); err != nil {
			return fmt.Errorf("failed to complete booking: %w", err)
		}
	}

	// Update creator stats
	if _, err := s.DB.Exec(ctx, `
        UPDATE creator_profiles cp
        SET completed_bookings = completed_bookings + 1
        FROM bookings b
        WHERE b.id = $1 AND cp.id = b.profile_id
    `, bookingID); err != nil {
		fmt.Printf("WARNING: failed to update completed count for booking %s: %v\n", bookingID, err)
	}

	return nil
}

// SubmitReview — client leaves a review after completion
type SubmitReviewInput struct {
	TenantID      string
	BookingID     string
	ReviewerEmail string
	ReviewerName  string
	Rating        int
	Body          string
}

func (s *BookingService) SubmitReview(ctx context.Context, input SubmitReviewInput) error {
	// Verify booking is completed by this client
	var profileID string
	err := s.DB.QueryRow(ctx, `
        SELECT profile_id FROM bookings
        WHERE id = $1 AND tenant_id = $2
          AND client_email = $3
          AND status = 'completed'
    `, input.BookingID, input.TenantID, input.ReviewerEmail).Scan(&profileID)
	if err != nil {
		return fmt.Errorf("booking not found or not completed: %w", err)
	}

	// Insert review
	_, err = s.DB.Exec(ctx, `
        INSERT INTO reviews (booking_id, profile_id, reviewer_email, reviewer_name, rating, body)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (booking_id) DO UPDATE
        SET rating = $5, body = $6
    `, input.BookingID, profileID, input.ReviewerEmail, input.ReviewerName, input.Rating, input.Body)
	if err != nil {
		return err
	}

	// Recalculate avg rating on profile
	_, err = s.DB.Exec(ctx, `
        UPDATE creator_profiles
        SET avg_rating = (
            SELECT AVG(rating)::NUMERIC(3,2)
            FROM reviews
            WHERE profile_id = $1 AND is_published = true
        ),
        total_reviews = (
            SELECT COUNT(*)
            FROM reviews
            WHERE profile_id = $1 AND is_published = true
        )
        WHERE id = $1
    `, profileID)
	if err != nil {
		return fmt.Errorf("failed to update profile rating: %w", err)
	}

	return nil
}
