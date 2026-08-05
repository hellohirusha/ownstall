package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/pkg/ai"
)

// RecommendationService indexes products as vectors and answers
// "customers also liked" queries with pgvector cosine search.
type RecommendationService struct {
	DB       *pgxpool.Pool
	Embedder ai.Embedder
}

// setTenantContext sets the RLS context variable so Postgres
// enforces tenant isolation on every query
func (s *RecommendationService) setTenantContext(ctx context.Context, tenantID string) error {
	_, err := s.DB.Exec(ctx,
		"SELECT set_config('app.current_tenant_id', $1, true)",
		tenantID,
	)
	return err
}

func (s *RecommendationService) embedder() ai.Embedder {
	if s.Embedder == nil {
		return ai.LexicalEmbedder{}
	}
	return s.Embedder
}

// RecommendedProduct is one similar product, ordered by similarity.
type RecommendedProduct struct {
	ID              string
	Name            string
	Slug            string
	BasePrice       float64
	ImageURL        *string
	SimilarityScore float64
}

// IndexProduct generates and stores the embedding for one product.
// Call it whenever a product's name, tags or description changes —
// a stale vector silently degrades every recommendation it appears in.
func (s *RecommendationService) IndexProduct(ctx context.Context, tenantID, productID string) error {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return fmt.Errorf("failed to set tenant context: %w", err)
	}

	var name, description string
	var tags []string
	var price float64

	err := s.DB.QueryRow(ctx, `
        SELECT name, COALESCE(description, ''), tags, base_price
        FROM products
        WHERE id = $1 AND tenant_id = $2
    `, productID, tenantID).Scan(&name, &description, &tags, &price)
	if err != nil {
		return fmt.Errorf("product %s not found: %w", productID, err)
	}

	doc := ai.Document{Title: name, Tags: tags, Body: description}

	embedder := s.embedder()
	vec, err := embedder.Embed(ctx, doc)
	if err != nil {
		return fmt.Errorf("embedding generation failed: %w", err)
	}

	// source_text is kept for debugging: when a recommendation looks
	// wrong, this is what the vector was actually built from.
	sourceText := fmt.Sprintf("Product: %s\nTags: %v\nDescription: %s", name, tags, description)

	_, err = s.DB.Exec(ctx, `
        INSERT INTO product_embeddings (product_id, tenant_id, embedding, source_text, model)
        VALUES ($1, $2, $3::vector, $4, $5)
        ON CONFLICT (product_id) DO UPDATE
        SET embedding   = EXCLUDED.embedding,
            source_text = EXCLUDED.source_text,
            model       = EXCLUDED.model,
            created_at  = NOW()
    `, productID, tenantID, ai.VectorLiteral(vec), sourceText, embedder.Model())
	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	return nil
}

// IndexAllProducts re-indexes every active product for a tenant and
// returns how many were indexed.
func (s *RecommendationService) IndexAllProducts(ctx context.Context, tenantID string) (int, error) {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return 0, fmt.Errorf("failed to set tenant context: %w", err)
	}

	rows, err := s.DB.Query(ctx, `
        SELECT id FROM products
        WHERE tenant_id = $1 AND status = 'active'
        ORDER BY created_at
    `, tenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to list products: %w", err)
	}

	var productIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("failed to scan product id: %w", err)
		}
		productIDs = append(productIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("failed to read products: %w", err)
	}

	indexed := 0
	for _, id := range productIDs {
		if err := s.IndexProduct(ctx, tenantID, id); err != nil {
			// One bad product should not abandon the rest of the catalogue
			fmt.Printf("recommendations: failed to index product %s: %v\n", id, err)
			continue
		}
		indexed++
	}

	return indexed, nil
}

// GetRecommendations returns the products most similar to the given
// one. Returns an empty slice (not an error) when the product has no
// embedding yet, so a storefront that has never been indexed simply
// shows no recommendations.
func (s *RecommendationService) GetRecommendations(
	ctx context.Context, tenantID, productID string, limit int,
) ([]*RecommendedProduct, error) {
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	// The target vector is fetched separately rather than joined as a
	// CTE: pgvector can only use the HNSW index when one side of <=>
	// is a parameter, and a subquery would force a sequential scan.
	var target string
	err := s.DB.QueryRow(ctx, `
        SELECT embedding::text
        FROM product_embeddings
        WHERE product_id = $1 AND tenant_id = $2
    `, productID, tenantID).Scan(&target)
	if err != nil {
		return []*RecommendedProduct{}, nil
	}

	rows, err := s.DB.Query(ctx, `
        SELECT
            p.id, p.name, p.slug, p.base_price,
            (SELECT url FROM product_images pi
             WHERE pi.product_id = p.id
             ORDER BY pi.position ASC LIMIT 1) AS image_url,
            -- Cosine distance → similarity, so higher is more alike
            1 - (pe.embedding <=> $1::vector) AS similarity_score
        FROM product_embeddings pe
        JOIN products p ON p.id = pe.product_id
        WHERE pe.tenant_id = $2
          AND pe.product_id <> $3
          AND p.status = 'active'
        ORDER BY pe.embedding <=> $1::vector
        LIMIT $4
    `, target, tenantID, productID, limit)
	if err != nil {
		return nil, fmt.Errorf("recommendation query failed: %w", err)
	}
	defer rows.Close()

	recommendations := make([]*RecommendedProduct, 0, limit)
	for rows.Next() {
		var r RecommendedProduct
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Slug, &r.BasePrice, &r.ImageURL, &r.SimilarityScore,
		); err != nil {
			return nil, fmt.Errorf("failed to scan recommendation: %w", err)
		}
		recommendations = append(recommendations, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read recommendations: %w", err)
	}

	return recommendations, nil
}

// CountMissingEmbeddings reports how many active products have no
// vector yet — the number the admin dashboard offers to index.
func (s *RecommendationService) CountMissingEmbeddings(ctx context.Context, tenantID string) (int, error) {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return 0, fmt.Errorf("failed to set tenant context: %w", err)
	}

	var count int
	err := s.DB.QueryRow(ctx, `
        SELECT COUNT(*)
        FROM products p
        LEFT JOIN product_embeddings pe ON pe.product_id = p.id
        WHERE p.tenant_id = $1 AND p.status = 'active' AND pe.id IS NULL
    `, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count missing embeddings: %w", err)
	}

	return count, nil
}
