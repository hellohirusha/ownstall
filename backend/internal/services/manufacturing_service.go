package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/pkg/telemetry"
)

type ManufacturingService struct {
	DB *pgxpool.Pool
}

// ProductionStats summarises pipeline throughput for the dashboard
type ProductionStats struct {
	TotalInProduction int
	TotalShipped      int
	AvgTimeHours      float64
	TodayQueued       int
	TodayShipped      int
}

// productionStages is the happy-path order of the pipeline
var productionStages = []string{
	"art_review", "printing", "cutting",
	"quality_check", "packaging", "shipped",
}

// stageTimestampColumn maps a stage to the column stamped on entry
var stageTimestampColumn = map[string]string{
	"art_review":    "art_review_at",
	"printing":      "printing_at",
	"cutting":       "cutting_at",
	"quality_check": "quality_check_at",
	"packaging":     "packaging_at",
	"shipped":       "shipped_at",
}

// webhookClient bounds outbound webhook calls so a slow endpoint
// cannot pile up goroutines
var webhookClient = &http.Client{Timeout: 10 * time.Second}

// EnqueueOrder adds a paid order to the production queue.
// Called from the Stripe webhook handler when order.status → 'paid'.
func (s *ManufacturingService) EnqueueOrder(ctx context.Context, tenantID, orderID string) error {
	// Calculate estimated completion based on current capacity
	estimatedCompletion := s.calculateEstimatedCompletion(ctx, tenantID)

	var queueID string
	err := s.DB.QueryRow(ctx, `
        INSERT INTO production_queue
            (tenant_id, order_id, status, priority, estimated_completion_at)
        VALUES ($1, $2, 'queued', 5, $3)
        ON CONFLICT (order_id) DO NOTHING
        RETURNING id
    `, tenantID, orderID, estimatedCompletion).Scan(&queueID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already queued — enqueueing twice is not an error
			return nil
		}
		return fmt.Errorf("failed to enqueue order %s: %w", orderID, err)
	}

	// Reserve capacity on the estimated completion date
	if _, err := s.DB.Exec(ctx, `
        INSERT INTO production_capacity (tenant_id, date, max_orders, current_orders)
        VALUES ($1, $2, 100, 1)
        ON CONFLICT (tenant_id, date)
        DO UPDATE SET current_orders = production_capacity.current_orders + 1
    `, tenantID, estimatedCompletion.Format("2006-01-02")); err != nil {
		fmt.Printf("WARNING: failed to reserve capacity for order %s: %v\n", orderID, err)
	}

	// Log event against the queue row (not the order)
	s.logEvent(ctx, tenantID, queueID, "", "queued", "", "")

	fmt.Printf("Order %s queued for production. Estimated: %s\n",
		orderID, estimatedCompletion.Format("Jan 2, 2006"))
	return nil
}

// AdvanceStatus moves a production item to the next stage.
// In production, this is called by real manufacturing machines.
// For demo, we call it manually or via the simulator.
func (s *ManufacturingService) AdvanceStatus(ctx context.Context,
	tenantID, queueID, newStatus, machineID, operatorID string,
) error {
	// Get current status
	var currentStatus string
	err := s.DB.QueryRow(ctx, `
        SELECT status FROM production_queue
        WHERE id = $1 AND tenant_id = $2
    `, queueID, tenantID).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("queue item %s not found", queueID)
	}

	// Build the update — stamp the stage-specific timestamp.
	// COALESCE keeps a previously recorded machine/operator when this
	// transition does not name one.
	query := `UPDATE production_queue
              SET status = $1,
                  machine_id = COALESCE($2, machine_id),
                  operator_id = COALESCE($3, operator_id)`
	if tsCol, ok := stageTimestampColumn[newStatus]; ok {
		query += fmt.Sprintf(", %s = NOW()", tsCol)
	}
	query += " WHERE id = $4 AND tenant_id = $5"

	if _, err := s.DB.Exec(ctx, query,
		newStatus, nullableString(machineID), nullableString(operatorID),
		queueID, tenantID,
	); err != nil {
		return fmt.Errorf("failed to advance queue item %s: %w", queueID, err)
	}

	// Log the event
	s.logEvent(ctx, tenantID, queueID, currentStatus, newStatus, machineID, operatorID)
	telemetry.ProductionOrdersTotal.WithLabelValues(newStatus).Inc()

	// When order ships, update the orders table and notify customer
	if newStatus == "shipped" {
		if _, err := s.DB.Exec(ctx, `
            UPDATE orders o
            SET status = 'shipped', shipped_at = NOW()
            FROM production_queue pq
            WHERE pq.id = $1 AND o.id = pq.order_id
        `, queueID); err != nil {
			fmt.Printf("WARNING: failed to mark order shipped for queue %s: %v\n", queueID, err)
		}

		// Detached from the caller's context — the request may end first
		go s.sendShippingNotification(context.Background(), tenantID, queueID)
	}

	// Fire outbound webhooks to any configured endpoints
	go s.fireWebhooks(context.Background(), tenantID, queueID, currentStatus, newStatus)

	return nil
}

// SimulateProduction advances every in-flight order one stage, with a
// short random delay so the dashboard shows movement.
// In a real system, operators and machines update status manually.
func (s *ManufacturingService) SimulateProduction(ctx context.Context, tenantID string) error {
	rows, err := s.DB.Query(ctx, `
        SELECT id, status FROM production_queue
        WHERE tenant_id = $1 AND status NOT IN ('shipped', 'cancelled', 'on_hold')
        ORDER BY priority DESC, created_at ASC
        LIMIT 20
    `, tenantID)
	if err != nil {
		return err
	}
	defer rows.Close()

	machines := []string{"PRINTER-01", "PRINTER-02", "CUTTER-01", "QA-STATION-01"}

	type advance struct {
		queueID string
		next    string
		machine string
	}
	var pending []advance

	for rows.Next() {
		var queueID, currentStatus string
		if err := rows.Scan(&queueID, &currentStatus); err != nil {
			return fmt.Errorf("failed to scan queue row: %w", err)
		}

		next := nextStage(currentStatus)
		if next == "" {
			continue
		}

		pending = append(pending, advance{
			queueID: queueID,
			next:    next,
			machine: machines[rand.Intn(len(machines))],
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read production queue: %w", err)
	}

	// Rows must be closed before the goroutines write, and the request
	// context dies with the response — so run detached
	for _, a := range pending {
		go func(a advance) {
			time.Sleep(time.Duration(rand.Intn(9)+1) * time.Second)
			if err := s.AdvanceStatus(context.Background(), tenantID, a.queueID, a.next, a.machine, ""); err != nil {
				fmt.Printf("simulate: failed to advance %s: %v\n", a.queueID, err)
			}
		}(a)
	}

	return nil
}

// nextStage returns the stage following current, or "" at the end
func nextStage(current string) string {
	if current == "queued" {
		return productionStages[0]
	}
	for i, stage := range productionStages {
		if stage == current && i+1 < len(productionStages) {
			return productionStages[i+1]
		}
	}
	return ""
}

// GetProductionStats returns throughput and latency metrics
func (s *ManufacturingService) GetProductionStats(ctx context.Context, tenantID string) (*ProductionStats, error) {
	var stats ProductionStats

	if err := s.DB.QueryRow(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE status NOT IN ('shipped', 'cancelled')),
            COUNT(*) FILTER (WHERE status = 'shipped'),
            COALESCE(AVG(EXTRACT(EPOCH FROM (shipped_at - queued_at)) / 3600)
                     FILTER (WHERE status = 'shipped' AND shipped_at IS NOT NULL), 0),
            COUNT(*) FILTER (WHERE DATE(queued_at) = CURRENT_DATE),
            COUNT(*) FILTER (WHERE DATE(shipped_at) = CURRENT_DATE)
        FROM production_queue
        WHERE tenant_id = $1
    `, tenantID).Scan(
		&stats.TotalInProduction, &stats.TotalShipped, &stats.AvgTimeHours,
		&stats.TodayQueued, &stats.TodayShipped,
	); err != nil {
		return nil, fmt.Errorf("failed to query production stats: %w", err)
	}

	return &stats, nil
}

// fireWebhooks sends POST requests to all active webhook endpoints for this tenant
func (s *ManufacturingService) fireWebhooks(ctx context.Context,
	tenantID, queueID, fromStatus, toStatus string,
) {
	rows, err := s.DB.Query(ctx, `
        SELECT url, COALESCE(secret, '') FROM webhook_endpoints
        WHERE tenant_id = $1 AND is_active = true
          AND $2 = ANY(event_types)
    `, tenantID, "production.status_changed")
	if err != nil {
		fmt.Printf("ERROR: failed to load webhook endpoints: %v\n", err)
		return
	}
	defer rows.Close()

	payload, err := json.Marshal(map[string]interface{}{
		"event":       "production.status_changed",
		"queue_id":    queueID,
		"from_status": fromStatus,
		"to_status":   toStatus,
		"timestamp":   time.Now().Unix(),
	})
	if err != nil {
		fmt.Printf("ERROR: failed to marshal webhook payload: %v\n", err)
		return
	}

	type endpoint struct{ url, secret string }
	var endpoints []endpoint
	for rows.Next() {
		var e endpoint
		if err := rows.Scan(&e.url, &e.secret); err != nil {
			fmt.Printf("ERROR: failed to scan webhook endpoint: %v\n", err)
			return
		}
		endpoints = append(endpoints, e)
	}

	for _, e := range endpoints {
		go func(webhookURL, webhookSecret string) {
			req, err := http.NewRequestWithContext(context.Background(),
				http.MethodPost, webhookURL, bytes.NewReader(payload))
			if err != nil {
				fmt.Printf("Webhook to %s failed to build: %v\n", webhookURL, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			if webhookSecret != "" {
				mac := hmac.New(sha256.New, []byte(webhookSecret))
				mac.Write(payload)
				req.Header.Set("X-Ownstall-Signature",
					"sha256="+hex.EncodeToString(mac.Sum(nil)))
			}

			resp, err := webhookClient.Do(req)
			if err != nil {
				fmt.Printf("Webhook to %s failed: %v\n", webhookURL, err)
				return
			}
			defer func() { _ = resp.Body.Close() }()
			fmt.Printf("Webhook to %s: HTTP %d\n", webhookURL, resp.StatusCode)
		}(e.url, e.secret)
	}
}

func (s *ManufacturingService) logEvent(ctx context.Context,
	tenantID, queueID, from, to, machineID, operatorID string,
) {
	if _, err := s.DB.Exec(ctx, `
        INSERT INTO production_events
            (queue_id, tenant_id, from_status, to_status, machine_id, operator_id)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, queueID, tenantID, nullableString(from), to,
		nullableString(machineID), nullableString(operatorID)); err != nil {
		fmt.Printf("WARNING: failed to log production event for %s: %v\n", queueID, err)
	}
}

// calculateEstimatedCompletion finds the first weekday in the next week
// that still has capacity left
func (s *ManufacturingService) calculateEstimatedCompletion(ctx context.Context, tenantID string) time.Time {
	for i := 1; i <= 7; i++ {
		date := time.Now().AddDate(0, 0, i)
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue // Skip weekends
		}

		// Defaults apply when no capacity row exists for the day yet
		currentOrders, maxOrders := 0, 100
		err := s.DB.QueryRow(ctx, `
            SELECT current_orders, max_orders, is_holiday
            FROM production_capacity
            WHERE tenant_id = $1 AND date = $2
        `, tenantID, date.Format("2006-01-02")).Scan(&currentOrders, &maxOrders, new(bool))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			fmt.Printf("WARNING: capacity lookup failed for %s: %v\n", date.Format("2006-01-02"), err)
		}

		if currentOrders < maxOrders {
			return date
		}
	}
	return time.Now().AddDate(0, 0, 7) // Fallback: 7 days
}

func (s *ManufacturingService) sendShippingNotification(ctx context.Context, tenantID, queueID string) {
	var orderID string
	if err := s.DB.QueryRow(ctx,
		"SELECT order_id FROM production_queue WHERE id = $1", queueID,
	).Scan(&orderID); err != nil {
		fmt.Printf("WARNING: shipping notification lookup failed for %s: %v\n", queueID, err)
		return
	}
	fmt.Printf("TODO: Send shipping notification for order %s\n", orderID)
	// emailSvc.SendTransactional(ctx, tenantID, "shipping_notification", ...)
}
