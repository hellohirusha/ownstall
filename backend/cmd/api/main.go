package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"

	"github.com/hellohirusha/ownstall/graph"
	"github.com/hellohirusha/ownstall/internal/handlers"
	appMiddleware "github.com/hellohirusha/ownstall/internal/middleware"
	"github.com/hellohirusha/ownstall/internal/services"
	"github.com/hellohirusha/ownstall/pkg/database"
	"github.com/hellohirusha/ownstall/pkg/queue"
	"github.com/hellohirusha/ownstall/pkg/storage"
)

func main() {
	if os.Getenv("ENVIRONMENT") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found — using system environment variables")
		}
	}

	db, err := database.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(30 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", os.Getenv("FRONTEND_URL")},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","version":"0.1.0"}`))
	})

	queueClient, err := queue.NewClient()
	if err != nil {
		log.Printf("WARNING: Redis unavailable — order emails disabled: %v", err)
	}
	emailService := &services.EmailService{DB: db, Queue: queueClient}
	authHandler := &handlers.AuthHandler{DB: db, Email: emailService}
	checkoutHandler := &handlers.CheckoutHandler{DB: db, Email: emailService}
	uploadHandler := &handlers.UploadHandler{Storage: storage.NewCloudinaryService()}

	// Campaign scheduler — dispatches due campaigns every 60 seconds
	campaignService := &services.CampaignService{DB: db, Queue: queueClient}
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := campaignService.ProcessDueCampaigns(context.Background()); err != nil {
				log.Printf("campaign scheduler: %v", err)
			}
		}
	}()

	// SLA checker — alerts on tickets nearing/past first-response deadline
	slaService := &services.SLAService{DB: db}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := slaService.CheckSLABreaches(context.Background()); err != nil {
				log.Printf("SLA check: %v", err)
			}
		}
	}()

	r.Route("/api", func(r chi.Router) {
		r.Post("/signup", authHandler.Signup)
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.AuthRequired)
			r.Post("/checkout/session", checkoutHandler.CreateCheckoutSession)
			r.Post("/upload/product-image", uploadHandler.UploadProductImage)
		})
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"message":"Ownstall API"}`))
		})
	})

	// Webhooks - NO auth middleware (Stripe calls these directly)
	r.Post("/webhooks/stripe", checkoutHandler.HandleStripeWebhook)

	// Email tracking - NO auth (mail clients load the pixel, Resend posts events)
	trackingHandler := &handlers.EmailTrackingHandler{DB: db}
	r.Get("/webhooks/email/open", trackingHandler.HandleOpen)
	r.Post("/webhooks/resend", trackingHandler.HandleResendWebhook)

	// Inbound support email - NO auth (Resend inbound routing posts here)
	ticketService := &services.TicketService{DB: db}
	inboundEmailHandler := &handlers.InboundEmailHandler{DB: db, TicketService: ticketService}
	r.Post("/webhooks/email/inbound", inboundEmailHandler.HandleInboundEmail)

	graphqlHandler := handler.NewDefaultServer(
		graph.NewExecutableSchema(graph.Config{
			Resolvers: &graph.Resolver{
				DB:             db,
				ProductService: &services.ProductService{DB: db},
				TicketService:  ticketService,
			},
		}),
	)

	if os.Getenv("ENVIRONMENT") != "production" {
		r.Handle("/playground", playground.Handler("GraphQL", "/query"))
	}
	r.With(appMiddleware.AuthOptional).Handle("/query", graphqlHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Server starting on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Forced shutdown: %v", err)
	}
	log.Println("Shutdown complete")
}
