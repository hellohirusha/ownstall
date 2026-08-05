package graph

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/internal/services"
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
}
