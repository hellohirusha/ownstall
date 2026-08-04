package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	appMiddleware "github.com/hellohirusha/ownstall/internal/middleware"
	"github.com/hellohirusha/ownstall/pkg/storage"
)

type UploadHandler struct {
	Storage *storage.CloudinaryService
}

// UploadProductImage handles multipart form uploads
// POST /api/upload/product-image
func (h *UploadHandler) UploadProductImage(w http.ResponseWriter, r *http.Request) {
	// Get tenant from JWT context (set by AuthRequired middleware)
	tenantID := appMiddleware.GetTenantID(r.Context())
	if tenantID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Parse multipart form (max 10MB per image)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, `{"error":"file too large — max 10MB"}`, http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, `{"error":"no image file in request"}`, http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	// Upload to Cloudinary in the tenant's folder
	folder := fmt.Sprintf("ownstall/products/%s", tenantID)
	result, err := h.Storage.UploadImage(r.Context(), file, folder)
	if err != nil {
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"url":    result.URL,
		"width":  result.Width,
		"height": result.Height,
	})
}

// GetPresignedURL returns upload credentials for direct browser-to-Cloudinary upload
// GET /api/upload/presigned?folder=products
func (h *UploadHandler) GetPresignedURL(w http.ResponseWriter, r *http.Request) {
	tenantID := appMiddleware.GetTenantID(r.Context())
	folder := fmt.Sprintf("ownstall/products/%s", tenantID)

	params := h.Storage.GeneratePresignedURL(folder)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(params)
}
