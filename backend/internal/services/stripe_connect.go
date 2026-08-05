package services

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/account"
	"github.com/stripe/stripe-go/v76/accountlink"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/transfer"
)

// StripeConnectService handles marketplace payments:
// creators onboard Stripe Express accounts, clients pay the platform,
// and funds are transferred to the creator when the booking completes.
type StripeConnectService struct {
	DB *pgxpool.Pool
}

// CreateConnectedAccount creates a Stripe Express account for a creator
// Returns the account ID to store on the creator_profiles table
func (s *StripeConnectService) CreateConnectedAccount(ctx context.Context,
	email string,
) (string, error) {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	acct, err := account.New(&stripe.AccountParams{
		Type:  stripe.String(string(stripe.AccountTypeExpress)),
		Email: stripe.String(email),
		Capabilities: &stripe.AccountCapabilitiesParams{
			Transfers: &stripe.AccountCapabilitiesTransfersParams{
				Requested: stripe.Bool(true),
			},
		},
		BusinessType: stripe.String("individual"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Stripe account: %w", err)
	}

	return acct.ID, nil
}

// GenerateOnboardingLink creates a URL that takes the creator through
// Stripe's hosted onboarding flow (they add bank details, verify identity)
func (s *StripeConnectService) GenerateOnboardingLink(ctx context.Context,
	stripeAccountID, profileID string,
) (string, error) {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	frontendURL := os.Getenv("FRONTEND_URL")

	link, err := accountlink.New(&stripe.AccountLinkParams{
		Account:    stripe.String(stripeAccountID),
		RefreshURL: stripe.String(fmt.Sprintf("%s/admin/hire/onboarding?refresh=true&profile=%s", frontendURL, profileID)),
		ReturnURL:  stripe.String(fmt.Sprintf("%s/admin/hire/onboarding?success=true&profile=%s", frontendURL, profileID)),
		Type:       stripe.String("account_onboarding"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create onboarding link: %w", err)
	}

	return link.URL, nil
}

// CreateBookingPayment creates a PaymentIntent with funds held on the platform
// (funds are NOT transferred to creator yet — that happens on completion)
func (s *StripeConnectService) CreateBookingPayment(ctx context.Context,
	bookingID string,
	amountUSD float64,
	creatorStripeAccountID string,
	customerEmail string,
) (string, string, error) {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	// Platform fee: 10% of booking value
	platformFeePercent := 0.10
	platformFeeAmount := int64(amountUSD * platformFeePercent * 100) // cents

	totalAmount := int64(amountUSD * 100) // cents

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(totalAmount),
		Currency: stripe.String("usd"),
		// Charge lands on the platform account — the creator's share is
		// transferred manually when the client approves delivery
		CaptureMethod: stripe.String(string(stripe.PaymentIntentCaptureMethodAutomatic)),
		Metadata: map[string]string{
			"booking_id":          bookingID,
			"creator_account_id":  creatorStripeAccountID,
			"platform_fee_amount": fmt.Sprintf("%d", platformFeeAmount),
		},
		ReceiptEmail: stripe.String(customerEmail),
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return "", "", fmt.Errorf("failed to create payment intent: %w", err)
	}

	return pi.ID, pi.ClientSecret, nil
}

// ReleaseEscrow transfers funds from platform to creator when booking completes
// Called when client marks booking as "completed"
func (s *StripeConnectService) ReleaseEscrow(ctx context.Context, bookingID string) error {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	// Load booking details
	var booking struct {
		AgreedPrice           float64
		PlatformFee           float64
		CreatorPayout         float64
		StripePaymentIntentID string
		StripeAccountID       string
	}

	err := s.DB.QueryRow(ctx, `
        SELECT
            b.agreed_price,
            b.platform_fee,
            b.creator_payout,
            COALESCE(b.stripe_payment_intent_id, ''),
            COALESCE(cp.stripe_account_id, '')
        FROM bookings b
        JOIN creator_profiles cp ON cp.id = b.profile_id
        WHERE b.id = $1
    `, bookingID).Scan(
		&booking.AgreedPrice, &booking.PlatformFee,
		&booking.CreatorPayout, &booking.StripePaymentIntentID,
		&booking.StripeAccountID,
	)
	if err != nil {
		return fmt.Errorf("booking %s not found: %w", bookingID, err)
	}

	if booking.StripeAccountID == "" {
		return fmt.Errorf("creator has no Stripe account connected")
	}
	if booking.StripePaymentIntentID == "" {
		return fmt.Errorf("booking has no payment to release")
	}

	// A transfer's source must be the underlying charge, not the
	// PaymentIntent — and the payment must actually have succeeded
	pi, err := paymentintent.Get(booking.StripePaymentIntentID, nil)
	if err != nil {
		return fmt.Errorf("failed to load payment intent: %w", err)
	}
	if pi.Status != stripe.PaymentIntentStatusSucceeded {
		return fmt.Errorf("payment not completed (status %s) — cannot release escrow", pi.Status)
	}
	if pi.LatestCharge == nil || pi.LatestCharge.ID == "" {
		return fmt.Errorf("payment intent has no charge to transfer from")
	}

	// Transfer creator's share to their Stripe account
	payoutAmount := int64(booking.CreatorPayout * 100) // cents

	t, err := transfer.New(&stripe.TransferParams{
		Amount:            stripe.Int64(payoutAmount),
		Currency:          stripe.String("usd"),
		Destination:       stripe.String(booking.StripeAccountID),
		SourceTransaction: stripe.String(pi.LatestCharge.ID),
		Metadata: map[string]string{
			"booking_id": bookingID,
		},
	})
	if err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}

	// Update booking with transfer ID and mark payment released
	_, err = s.DB.Exec(ctx, `
        UPDATE bookings
        SET stripe_transfer_id = $1,
            payment_status = 'released',
            completed_at = NOW(),
            status = 'completed'
        WHERE id = $2
    `, t.ID, bookingID)

	return err
}
