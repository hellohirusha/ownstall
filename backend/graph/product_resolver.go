package graph

import (
	"github.com/google/uuid"

	"github.com/hellohirusha/ownstall/graph/model"
	"github.com/hellohirusha/ownstall/internal/models"
)

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// parseUUID converts a database string ID into a uuid.UUID, returning
// uuid.Nil if the value is not a valid UUID.
func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

// modelToGraphQL maps an internal product model onto the generated
// GraphQL type, including its images and variants.
func modelToGraphQL(p *models.Product) *model.Product {
	if p == nil {
		return nil
	}

	out := &model.Product{
		ID:           parseUUID(p.ID),
		TenantID:     parseUUID(p.TenantID),
		Name:         p.Name,
		Slug:         p.Slug,
		Description:  p.Description,
		ShortDesc:    p.ShortDesc,
		BasePrice:    p.BasePrice,
		ComparePrice: p.ComparePrice,
		Status:       p.Status,
		IsFeatured:   p.IsFeatured,
		Tags:         p.Tags,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}

	for _, img := range p.Images {
		out.Images = append(out.Images, &model.ProductImage{
			ID:       parseUUID(img.ID),
			URL:      img.URL,
			AltText:  img.AltText,
			Position: int32(img.Position),
		})
	}

	for i := range p.Variants {
		v := p.Variants[i]
		out.Variants = append(out.Variants, &model.ProductVariant{
			ID:            parseUUID(v.ID),
			Sku:           v.SKU,
			Title:         v.Title,
			Option1Name:   v.Option1Name,
			Option1Value:  v.Option1Value,
			Option2Name:   v.Option2Name,
			Option2Value:  v.Option2Value,
			Price:         v.Price,
			ComparePrice:  v.ComparePrice,
			StockQuantity: int32(v.StockQuantity),
			IsInStock:     v.IsInStock(),
			IsActive:      v.IsActive,
			ImageURL:      v.ImageURL,
		})
	}

	return out
}
