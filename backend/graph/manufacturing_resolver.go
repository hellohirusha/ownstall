package graph

import (
	"context"
	"fmt"

	"github.com/hellohirusha/ownstall/graph/model"
)

// productionQueueColumns is the shared SELECT list for queue items.
const productionQueueColumns = `
    id, status, priority, machine_id, notes,
    estimated_completion_at, queued_at, shipped_at`

// scanProductionQueueItem maps one row (selected with productionQueueColumns)
// onto the generated GraphQL type. The order is loaded by the field resolver.
func scanProductionQueueItem(row interface{ Scan(dest ...any) error }) (*model.ProductionQueueItem, error) {
	var item model.ProductionQueueItem
	var id string
	if err := row.Scan(
		&id, &item.Status, &item.Priority, &item.MachineID, &item.Notes,
		&item.EstimatedCompletionAt, &item.QueuedAt, &item.ShippedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan production queue item: %w", err)
	}
	item.ID = parseUUID(id)
	return &item, nil
}

// loadOrder fetches one tenant-owned order with its line items.
func (r *Resolver) loadOrder(ctx context.Context, tenantID, orderID string) (*model.Order, error) {
	var o model.Order
	var id string
	if err := r.DB.QueryRow(ctx, `
        SELECT id, status, total, customer_email, customer_name, created_at, paid_at
        FROM orders
        WHERE id = $1 AND tenant_id = $2
    `, orderID, tenantID).Scan(
		&id, &o.Status, &o.Total, &o.CustomerEmail, &o.CustomerName,
		&o.CreatedAt, &o.PaidAt,
	); err != nil {
		return nil, err
	}
	o.ID = parseUUID(id)

	rows, err := r.DB.Query(ctx, `
        SELECT id, product_name, variant_title, quantity,
               unit_price, total_price, image_url
        FROM order_items
        WHERE order_id = $1
        ORDER BY created_at ASC
    `, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query order items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it model.OrderItem
		var itemID string
		if err := rows.Scan(
			&itemID, &it.ProductName, &it.VariantTitle, &it.Quantity,
			&it.UnitPrice, &it.TotalPrice, &it.ImageURL,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		it.ID = parseUUID(itemID)
		o.Items = append(o.Items, &it)
	}

	return &o, nil
}

// fetchProductionQueueItem loads a single tenant-owned queue item.
func (r *Resolver) fetchProductionQueueItem(ctx context.Context, tenantID, queueID string) (*model.ProductionQueueItem, error) {
	row := r.DB.QueryRow(ctx,
		"SELECT "+productionQueueColumns+` FROM production_queue
         WHERE id = $1 AND tenant_id = $2`, queueID, tenantID)
	return scanProductionQueueItem(row)
}
