package graph

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/internal/services"
	"github.com/hellohirusha/ownstall/pkg/ai"
)

// This file will not be regenerated automatically.
// It serves as dependency injection for your app.

type Resolver struct {
	DB             *pgxpool.Pool
	ProductService *services.ProductService
	TicketService  *services.TicketService
	BookingService *services.BookingService
	StripeConnect  *services.StripeConnectService

	ManufacturingService *services.ManufacturingService

	AI              *ai.Client
	CopyGenerator   *services.CopyGeneratorService
	Recommendations *services.RecommendationService
	AutoReply       *services.AutoReplyService
	ImageQA         *services.ImageQAService
}
