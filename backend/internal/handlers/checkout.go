package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"

	appMiddleware "github.com/hellohirusha/ownstall/internal/middleware"
	"github.com/hellohirusha/ownstall/internal/services"
	"github.com/hellohirusha/ownstall/pkg/telemetry"
)

type CheckoutHandler struct {
	DB            *pgxpool.Pool
	Email         *services.EmailService
	Manufacturing *services.ManufacturingService
	Push          *services.PushService
}

// CartItem represents a single item passed from the frontend
type CartItem struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

// CreateCheckoutSession creates a Stripe checkout session and returns the URL
// POST /api/checkout/session
func (h *CheckoutHandler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	// The buyer is whoever is shopping — a signed-in shopper or a guest.
	// Crucially the tenant is NOT taken from the caller's token: doing that
	// meant only a signed-in seller could check out, and the order landed on
	// whatever stall that seller happened to own. The stall being bought from
	// is a property of the cart, so it is derived from the cart.
	buyerID := appMiddleware.GetBuyerID(r.Context())

	var req struct {
		Items         []CartItem `json:"items"`
		SuccessURL    string     `json:"success_url"`
		CancelURL     string     `json:"cancel_url"`
		CustomerEmail string     `json:"customer_email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if len(req.Items) == 0 {
		http.Error(w, `{"error":"cart is empty"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.CustomerEmail) == "" {
		http.Error(w, `{"error":"an email address is required"}`, http.StatusBadRequest)
		return
	}

	tenantID, err := h.resolveCartTenant(r.Context(), req.Items)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Initialize Stripe with secret key
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	// Build Stripe line items from cart
	var lineItems []*stripe.CheckoutSessionLineItemParams

	for _, item := range req.Items {
		// Look up variant price and name from database
		var variantTitle, productName string
		var price float64

		err := h.DB.QueryRow(r.Context(), `
            SELECT
                v.title,
                v.price,
                p.name
            FROM product_variants v
            JOIN products p ON p.id = v.product_id
            WHERE v.id = $1
        `, item.VariantID).Scan(&variantTitle, &price, &productName)

		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"variant %s not found"}`, item.VariantID), http.StatusBadRequest)
			return
		}

		name := productName
		if variantTitle != "Default" {
			name = fmt.Sprintf("%s — %s", productName, variantTitle)
		}

		lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency: stripe.String("usd"),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String(name),
				},
				// Stripe uses cents (multiply by 100)
				UnitAmount: stripe.Int64(int64(price * 100)),
			},
			Quantity: stripe.Int64(int64(item.Quantity)),
		})
	}

	// Frontend URLs — passed in request or use defaults
	successURL := req.SuccessURL
	cancelURL := req.CancelURL
	if successURL == "" {
		successURL = fmt.Sprintf("%s/order/success?session_id={CHECKOUT_SESSION_ID}", os.Getenv("FRONTEND_URL"))
	}
	if cancelURL == "" {
		cancelURL = fmt.Sprintf("%s/cart", os.Getenv("FRONTEND_URL"))
	}

	// Create the Stripe checkout session
	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems:          lineItems,
		Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:         stripe.String(successURL),
		CancelURL:          stripe.String(cancelURL),
		CustomerEmail:      stripe.String(req.CustomerEmail),

		// Store our internal IDs in Stripe metadata
		// We read these in the webhook handler
		Metadata: map[string]string{
			"tenant_id": tenantID,
			"buyer_id":  buyerID,
		},
	}

	// Create a pending order in our database BEFORE Stripe session
	// This way we can match the webhook back to an order
	orderID, err := h.createPendingOrder(r.Context(), tenantID, buyerID, req.CustomerEmail, req.Items)
	if err != nil {
		http.Error(w, `{"error":"failed to create order"}`, http.StatusInternalServerError)
		return
	}

	params.Metadata["order_id"] = orderID

	s, err := session.New(params)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"stripe error: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Update order with Stripe session ID
	if _, err := h.DB.Exec(r.Context(),
		"UPDATE orders SET stripe_session_id = $1 WHERE id = $2",
		s.ID, orderID,
	); err != nil {
		fmt.Printf("ERROR: failed to store session id on order %s: %v\n", orderID, err)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"session_id":   s.ID,
		"checkout_url": s.URL,
		"order_id":     orderID,
	})
}

// resolveCartTenant works out which stall a cart belongs to, and refuses the
// checkout if that stall is not open for business.
//
// A cart may only contain items from one stall: each order belongs to a single
// tenant, and payment goes to a single seller. Mixing stalls is rejected here
// rather than silently attributing the whole order to the first one.
func (h *CheckoutHandler) resolveCartTenant(ctx context.Context, items []CartItem) (string, error) {
	variantIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.Quantity <= 0 {
			return "", fmt.Errorf("quantity must be at least 1")
		}
		variantIDs = append(variantIDs, item.VariantID)
	}

	rows, err := h.DB.Query(ctx, `
        SELECT DISTINCT p.tenant_id
        FROM product_variants v
        JOIN products p ON p.id = v.product_id
        WHERE v.id = ANY($1::uuid[])
    `, variantIDs)
	if err != nil {
		return "", fmt.Errorf("could not look up cart items")
	}
	defer rows.Close()

	var tenantIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("could not look up cart items")
		}
		tenantIDs = append(tenantIDs, id)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("could not look up cart items")
	}

	switch len(tenantIDs) {
	case 0:
		return "", fmt.Errorf("none of the items in your cart are available")
	case 1:
		// ok
	default:
		return "", fmt.Errorf("your cart mixes items from different stalls — please check out one stall at a time")
	}

	tenantID := tenantIDs[0]

	var status string
	var isActive, canAcceptOrders bool
	err = h.DB.QueryRow(ctx,
		`SELECT status, is_active, can_accept_orders FROM tenants WHERE id = $1`,
		tenantID,
	).Scan(&status, &isActive, &canAcceptOrders)
	if err != nil {
		return "", fmt.Errorf("this stall is not available")
	}

	// An unapproved or suspended stall must not be able to take money, even
	// if someone kept a direct link to one of its products.
	if status != "approved" || !isActive || !canAcceptOrders {
		return "", fmt.Errorf("this stall is not currently accepting orders")
	}

	return tenantID, nil
}

// createPendingOrder creates an order (status "pending") plus its order_items
// in a single transaction and returns the new order ID. The real payment state
// is set later by the Stripe webhook.
//
// buyerID is empty for a guest checkout, in which case the order is identified
// by customer_email alone.
func (h *CheckoutHandler) createPendingOrder(
	ctx context.Context,
	tenantID, buyerID, customerEmail string,
	items []CartItem,
) (string, error) {
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	// Rollback is a no-op after a successful commit
	defer func() { _ = tx.Rollback(ctx) }()

	// Set the tenant GUC so row-level security allows the inserts
	if _, err := tx.Exec(ctx,
		"SELECT set_config('app.current_tenant_id', $1, true)", tenantID,
	); err != nil {
		return "", fmt.Errorf("failed to set tenant context: %w", err)
	}

	// buyer_id is optional (guest checkout) — store NULL when absent.
	// user_id stays NULL: that column references the seller-side `users`
	// table, and a shopper is never a row in it.
	var buyerIDArg interface{}
	if buyerID != "" {
		buyerIDArg = buyerID
	}

	var orderID string
	err = tx.QueryRow(ctx, `
        INSERT INTO orders (tenant_id, buyer_id, customer_email, status, subtotal, total)
        VALUES ($1, $2, $3, 'pending', 0, 0)
        RETURNING id
    `, tenantID, buyerIDArg, customerEmail).Scan(&orderID)
	if err != nil {
		return "", fmt.Errorf("failed to insert order: %w", err)
	}

	var subtotal float64
	for _, item := range items {
		var (
			productID    string
			variantTitle string
			sku          string
			unitPrice    float64
			productName  string
			imageURL     *string
		)
		err = tx.QueryRow(ctx, `
            SELECT v.product_id, v.title, v.sku, v.price, p.name,
                   (SELECT url FROM product_images
                    WHERE product_id = p.id AND position = 0 LIMIT 1)
            FROM product_variants v
            JOIN products p ON p.id = v.product_id
            WHERE v.id = $1
        `, item.VariantID).Scan(
			&productID, &variantTitle, &sku, &unitPrice, &productName, &imageURL,
		)
		if err != nil {
			return "", fmt.Errorf("variant %s lookup failed: %w", item.VariantID, err)
		}

		lineTotal := unitPrice * float64(item.Quantity)
		subtotal += lineTotal

		_, err = tx.Exec(ctx, `
            INSERT INTO order_items
                (order_id, product_id, variant_id, product_name, variant_title,
                 sku, quantity, unit_price, total_price, image_url)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        `, orderID, productID, item.VariantID, productName, variantTitle,
			sku, item.Quantity, unitPrice, lineTotal, imageURL)
		if err != nil {
			return "", fmt.Errorf("failed to insert order item: %w", err)
		}
	}

	// Persist totals now that the line items are summed
	if _, err = tx.Exec(ctx,
		"UPDATE orders SET subtotal = $1, total = $2 WHERE id = $3",
		subtotal, subtotal, orderID,
	); err != nil {
		return "", fmt.Errorf("failed to update order totals: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return orderID, nil
}

// HandleStripeWebhook processes events from Stripe
// POST /webhooks/stripe
func (h *CheckoutHandler) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	// Read raw body — Stripe signature verification requires the raw bytes
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify webhook signature (prevents spoofed webhook calls)
	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	// IgnoreAPIVersionMismatch: the Stripe account/CLI sends events on a newer
	// API version than stripe-go pins; the fields we read are stable across them.
	event, err := webhook.ConstructEventWithOptions(
		payload,
		r.Header.Get("Stripe-Signature"),
		webhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
	)
	if err != nil {
		// Signature invalid — this could be an attack
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	// Handle the event type
	switch event.Type {

	case "checkout.session.completed":
		// Payment succeeded — update order to "paid"
		var s stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &s); err != nil {
			http.Error(w, "failed to parse session", http.StatusBadRequest)
			return
		}

		orderID := s.Metadata["order_id"]
		if orderID == "" {
			// Log but don't fail — old sessions may not have this
			fmt.Printf("WARNING: no order_id in Stripe session metadata %s\n", s.ID)
			w.WriteHeader(http.StatusOK)
			return
		}

		// PaymentIntent arrives as a struct reference — persist only its ID
		var paymentIntentID string
		if s.PaymentIntent != nil {
			paymentIntentID = s.PaymentIntent.ID
		}

		// Mark order as paid
		_, err := h.DB.Exec(r.Context(), `
            UPDATE orders
            SET status = 'paid',
                paid_at = NOW(),
                stripe_payment_id = $1,
                customer_email = $2
            WHERE id = $3 AND status = 'pending'
        `, paymentIntentID, s.CustomerEmail, orderID)

		if err != nil {
			fmt.Printf("ERROR: failed to update order %s: %v\n", orderID, err)
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		// Counted here rather than at session creation: this is the
		// point money actually moved.
		telemetry.OrdersTotal.WithLabelValues("paid").Inc()
		telemetry.OrderValueUSD.Observe(float64(s.AmountTotal) / 100)

		// Decrease inventory for each ordered variant
		go h.decrementInventory(orderID)

		tenantID := s.Metadata["tenant_id"]

		if h.Email != nil && h.Email.Queue != nil {
			go func() {
				if err := h.Email.SendOrderConfirmation(context.Background(), tenantID, orderID); err != nil {
					fmt.Printf("ERROR: order confirmation for %s failed: %v\n", orderID, err)
				}
			}()
		} else {
			fmt.Printf("WARNING: queue unavailable, skipping confirmation email for order %s\n", orderID)
		}

		// Hand the paid order to the production pipeline
		if h.Manufacturing != nil && tenantID != "" {
			go func() {
				if err := h.Manufacturing.EnqueueOrder(context.Background(), tenantID, orderID); err != nil {
					fmt.Printf("ERROR: failed to enqueue order %s for production: %v\n", orderID, err)
				}
			}()
		}

		// Push the sale to the merchant's phone
		if h.Push != nil && tenantID != "" {
			orderTotal := float64(s.AmountTotal) / 100
			customer := s.CustomerEmail
			go func() {
				if err := h.Push.NotifyTenant(context.Background(), tenantID,
					"New order 🎉",
					fmt.Sprintf("$%.2f from %s", orderTotal, customer),
					map[string]string{"type": "order", "order_id": orderID},
				); err != nil {
					fmt.Printf("ERROR: order push for %s failed: %v\n", orderID, err)
				}
			}()
		}

		fmt.Printf("Order %s paid via Stripe session %s\n", orderID, s.ID)

	case "checkout.session.expired":
		// Session expired without payment — mark order as failed
		var s stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &s); err != nil {
			fmt.Printf("ERROR: failed to parse expired session: %v\n", err)
			break
		}

		if orderID := s.Metadata["order_id"]; orderID != "" {
			if _, err := h.DB.Exec(r.Context(),
				"UPDATE orders SET status = 'failed' WHERE id = $1 AND status = 'pending'",
				orderID,
			); err != nil {
				fmt.Printf("ERROR: failed to mark order %s as failed: %v\n", orderID, err)
			}
		}

	case "payment_intent.payment_failed":
		// Payment explicitly failed
		fmt.Printf("Payment failed: %s\n", event.ID)

	default:
		// Log unknown events but return 200 (Stripe retries on non-200)
		fmt.Printf("Unhandled Stripe event: %s\n", event.Type)
	}

	// Always return 200 — Stripe retries events that receive non-200
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"received":true}`))
}

// decrementInventory reduces stock_quantity for all items in an order
func (h *CheckoutHandler) decrementInventory(orderID string) {
	ctx := context.Background()
	rows, err := h.DB.Query(ctx, `
        SELECT variant_id, quantity FROM order_items WHERE order_id = $1
    `, orderID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var variantID string
		var quantity int
		if err := rows.Scan(&variantID, &quantity); err != nil {
			fmt.Printf("ERROR: failed to scan order item: %v\n", err)
			continue
		}

		if _, err := h.DB.Exec(ctx, `
            UPDATE product_variants
            SET stock_quantity = GREATEST(stock_quantity - $1, 0)
            WHERE id = $2 AND track_inventory = true
        `, quantity, variantID); err != nil {
			fmt.Printf("ERROR: failed to decrement inventory for variant %s: %v\n", variantID, err)
		}
	}
}
