package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/internal/models"
)

type ProductService struct {
	DB *pgxpool.Pool
}

// setTenantContext sets the RLS context variable so Postgres
// enforces tenant isolation on every query
func (s *ProductService) setTenantContext(ctx context.Context, tenantID string) error {
	_, err := s.DB.Exec(ctx,
		"SELECT set_config('app.current_tenant_id', $1, true)",
		tenantID,
	)
	return err
}

// ListProducts returns all active products for a tenant with their
// primary image. Variants are loaded separately to avoid N+1 queries.
func (s *ProductService) ListProducts(ctx context.Context, tenantID string, status string) ([]*models.Product, error) {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	query := `
        SELECT
            p.id, p.tenant_id, p.category_id, p.name, p.slug,
            p.description, p.short_desc, p.base_price, p.compare_price,
            p.status, p.is_featured, p.tags, p.created_at, p.updated_at,
            -- Get primary image URL (position = 0) in the same query
            (SELECT url FROM product_images
             WHERE product_id = p.id AND position = 0
             LIMIT 1) AS primary_image_url
        FROM products p
        WHERE p.tenant_id = $1
    `

	args := []interface{}{tenantID}

	if status != "" {
		query += " AND p.status = $2"
		args = append(args, status)
	}

	query += " ORDER BY p.is_featured DESC, p.created_at DESC"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		var p models.Product
		var primaryImageURL *string

		err := rows.Scan(
			&p.ID, &p.TenantID, &p.CategoryID, &p.Name, &p.Slug,
			&p.Description, &p.ShortDesc, &p.BasePrice, &p.ComparePrice,
			&p.Status, &p.IsFeatured, &p.Tags, &p.CreatedAt, &p.UpdatedAt,
			&primaryImageURL,
		)
		if err != nil {
			return nil, err
		}

		// Add primary image as a single-item slice
		if primaryImageURL != nil {
			p.Images = []models.ProductImage{{URL: *primaryImageURL, Position: 0}}
		}

		products = append(products, &p)
	}

	if len(products) == 0 {
		return products, nil
	}

	// Batch-load variants so listings can show stock state without N+1 queries
	index := make(map[string]*models.Product, len(products))
	ids := make([]string, 0, len(products))
	for _, p := range products {
		index[p.ID] = p
		ids = append(ids, p.ID)
	}

	variantRows, err := s.DB.Query(ctx, `
        SELECT id, product_id, sku, title,
               option1_name, option1_value, option2_name, option2_value,
               price, compare_price, stock_quantity, track_inventory,
               allow_backorder, is_active, position, image_url,
               created_at, updated_at
        FROM product_variants
        WHERE product_id = ANY($1::uuid[])
        ORDER BY position ASC
    `, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to query variants: %w", err)
	}
	defer variantRows.Close()

	for variantRows.Next() {
		var v models.ProductVariant
		if err := variantRows.Scan(
			&v.ID, &v.ProductID, &v.SKU, &v.Title,
			&v.Option1Name, &v.Option1Value, &v.Option2Name, &v.Option2Value,
			&v.Price, &v.ComparePrice, &v.StockQuantity, &v.TrackInventory,
			&v.AllowBackorder, &v.IsActive, &v.Position, &v.ImageURL,
			&v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan variant row: %w", err)
		}
		if p, ok := index[v.ProductID]; ok {
			p.Variants = append(p.Variants, v)
		}
	}

	return products, nil
}

// GetProduct fetches a single product with all images and variants
func (s *ProductService) GetProduct(ctx context.Context, tenantID, productID string) (*models.Product, error) {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return nil, err
	}

	var p models.Product
	err := s.DB.QueryRow(ctx, `
        SELECT id, tenant_id, category_id, name, slug, description, short_desc,
               base_price, compare_price, status, is_featured, tags,
               ai_description, ai_quality_score, created_at, updated_at
        FROM products
        WHERE id = $1 AND tenant_id = $2
    `, productID, tenantID).Scan(
		&p.ID, &p.TenantID, &p.CategoryID, &p.Name, &p.Slug,
		&p.Description, &p.ShortDesc, &p.BasePrice, &p.ComparePrice,
		&p.Status, &p.IsFeatured, &p.Tags,
		&p.AIDescription, &p.AIQualityScore, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	// Load images
	imageRows, err := s.DB.Query(ctx, `
        SELECT id, product_id, url, alt_text, position, width, height, created_at
        FROM product_images
        WHERE product_id = $1
        ORDER BY position ASC
    `, productID)
	if err != nil {
		return nil, err
	}
	defer imageRows.Close()

	for imageRows.Next() {
		var img models.ProductImage
		if err := imageRows.Scan(
			&img.ID, &img.ProductID, &img.URL, &img.AltText,
			&img.Position, &img.Width, &img.Height, &img.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan image row: %w", err)
		}
		p.Images = append(p.Images, img)
	}

	// Load variants
	variantRows, err := s.DB.Query(ctx, `
        SELECT id, product_id, sku, title,
               option1_name, option1_value, option2_name, option2_value,
               price, compare_price, stock_quantity, track_inventory,
               allow_backorder, is_active, position, image_url,
               created_at, updated_at
        FROM product_variants
        WHERE product_id = $1
        ORDER BY position ASC
    `, productID)
	if err != nil {
		return nil, err
	}
	defer variantRows.Close()

	for variantRows.Next() {
		var v models.ProductVariant
		if err := variantRows.Scan(
			&v.ID, &v.ProductID, &v.SKU, &v.Title,
			&v.Option1Name, &v.Option1Value, &v.Option2Name, &v.Option2Value,
			&v.Price, &v.ComparePrice, &v.StockQuantity, &v.TrackInventory,
			&v.AllowBackorder, &v.IsActive, &v.Position, &v.ImageURL,
			&v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan variant row: %w", err)
		}
		p.Variants = append(p.Variants, v)
	}

	return &p, nil
}

// GetProductBySlug returns a product (with images and variants) by its
// URL slug — used by the public storefront product page
func (s *ProductService) GetProductBySlug(ctx context.Context, tenantID, slug string) (*models.Product, error) {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return nil, err
	}

	var productID string
	err := s.DB.QueryRow(ctx,
		"SELECT id FROM products WHERE slug = $1 AND tenant_id = $2",
		slug, tenantID,
	).Scan(&productID)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	return s.GetProduct(ctx, tenantID, productID)
}

// PublishProduct sets a product's status to active so it shows on the storefront
func (s *ProductService) PublishProduct(ctx context.Context, tenantID, productID string) (*models.Product, error) {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return nil, err
	}

	tag, err := s.DB.Exec(ctx,
		"UPDATE products SET status = 'active' WHERE id = $1 AND tenant_id = $2",
		productID, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to publish product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("product not found")
	}

	return s.GetProduct(ctx, tenantID, productID)
}

// CreateProduct inserts a new product + default variant in a transaction
type CreateProductInput struct {
	TenantID     string
	Name         string
	Description  string
	BasePrice    float64
	ComparePrice *float64
	Tags         []string
}

func (s *ProductService) CreateProduct(ctx context.Context, input CreateProductInput) (*models.Product, error) {
	if err := s.setTenantContext(ctx, input.TenantID); err != nil {
		return nil, err
	}

	// Generate URL-safe slug from name
	slug := slugify(input.Name)

	// tags is NOT NULL in the schema; coalesce nil to an empty array so an
	// omitted tags field stores '{}' instead of a NULL that violates the constraint
	if input.Tags == nil {
		input.Tags = []string{}
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// Rollback is a no-op after a successful commit
	defer func() { _ = tx.Rollback(ctx) }()

	var productID string
	err = tx.QueryRow(ctx, `
        INSERT INTO products
            (tenant_id, name, slug, description, base_price, compare_price, tags, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft')
        RETURNING id
    `, input.TenantID, input.Name, slug, input.Description,
		input.BasePrice, input.ComparePrice, input.Tags,
	).Scan(&productID)
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	// Create a default variant so the product is immediately purchasable
	sku := fmt.Sprintf("%s-default", strings.ToUpper(slug[:min(8, len(slug))]))
	_, err = tx.Exec(ctx, `
        INSERT INTO product_variants
            (product_id, sku, title, price, stock_quantity)
        VALUES ($1, $2, 'Default', $3, 100)
    `, productID, sku, input.BasePrice)
	if err != nil {
		return nil, fmt.Errorf("failed to create default variant: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.GetProduct(ctx, input.TenantID, productID)
}

// slugify converts "My Cool Product!" to "my-cool-product"
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	// Remove non-alphanumeric except dashes
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return strings.Trim(result.String(), "-")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
