package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/hellohirusha/ownstall/graph"
	"github.com/hellohirusha/ownstall/internal/handlers"
	appMiddleware "github.com/hellohirusha/ownstall/internal/middleware"
	"github.com/hellohirusha/ownstall/internal/services"
	"github.com/hellohirusha/ownstall/pkg/ai"
	"github.com/hellohirusha/ownstall/pkg/database"
	"github.com/hellohirusha/ownstall/pkg/queue"
	"github.com/hellohirusha/ownstall/pkg/storage"
	"github.com/hellohirusha/ownstall/pkg/telemetry"
)

// Default bounds for one client IP: generous enough that a storefront
// visitor browsing quickly never notices, tight enough to make
// scripted abuse expensive. Both are overridable by env so a limit can
// be tightened during an incident without a code change.
const (
	defaultRateLimitRequests = 300
	defaultRateLimitWindow   = time.Minute
)

func rateLimitSettings() (int, time.Duration) {
	requests := defaultRateLimitRequests
	if raw := os.Getenv("RATE_LIMIT_REQUESTS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			requests = parsed
		}
	}

	window := defaultRateLimitWindow
	if raw := os.Getenv("RATE_LIMIT_WINDOW_SECONDS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			window = time.Duration(parsed) * time.Second
		}
	}

	return requests, window
}

func main() {
	if os.Getenv("ENVIRONMENT") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found — using system environment variables")
		}
	}

	// Observability first, so anything that fails below is reported
	if err := telemetry.InitLogger(); err != nil {
		log.Fatalf("Failed to initialise logger: %v", err)
	}
	defer telemetry.SyncLogger()

	if err := telemetry.InitSentry(); err != nil {
		telemetry.Log.Warn("Sentry init failed: " + err.Error())
	}
	defer telemetry.FlushSentry()

	shutdownTracing, err := telemetry.InitTracing(context.Background())
	if err != nil {
		telemetry.Log.Warn("tracing init failed: " + err.Error())
		shutdownTracing = func(context.Context) {}
	}

	shutdownMetrics, err := telemetry.InitMetricsExport(context.Background())
	if err != nil {
		telemetry.Log.Warn("metrics export init failed: " + err.Error())
		shutdownMetrics = func(context.Context) {}
	}

	db, err := database.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Created before the router so the rate limiter can share this
	// connection pool rather than opening a second one.
	queueClient, err := queue.NewClient()
	if err != nil {
		telemetry.Log.Warn("Redis unavailable — order emails and rate limiting disabled: " + err.Error())
	}

	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.Recoverer)
	// Structured JSON logging replaces chi's line-per-request logger;
	// it carries status, duration, client IP and trace id.
	r.Use(telemetry.RequestLogger)
	r.Use(telemetry.SentryMiddleware)
	r.Use(telemetry.MetricsMiddleware)
	r.Use(appMiddleware.SecurityHeaders)
	r.Use(appMiddleware.RequestSanitizer)
	r.Use(chiMiddleware.Timeout(30 * time.Second))

	limitRequests, limitWindow := rateLimitSettings()
	rateLimiter := appMiddleware.NewRateLimiter(
		queueClient.Redis(), limitRequests, limitWindow,
	)
	r.Use(rateLimiter.RateLimit)

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

	// Prometheus scrape endpoint. Guarded by a shared token: it
	// exposes traffic shape and business counters, which is not
	// something to publish on an open port.
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		token := os.Getenv("METRICS_TOKEN")
		if token == "" || r.Header.Get("X-Metrics-Token") != token {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		telemetry.MetricsHandler().ServeHTTP(w, r)
	})
	emailService := &services.EmailService{DB: db, Queue: queueClient}
	manufacturingService := &services.ManufacturingService{DB: db}
	pushService := &services.PushService{DB: db}
	authHandler := &handlers.AuthHandler{DB: db, Email: emailService}
	checkoutHandler := &handlers.CheckoutHandler{
		DB:            db,
		Email:         emailService,
		Manufacturing: manufacturingService,
		Push:          pushService,
	}
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

	// Queue depth gauge — a growing backlog is the first sign the
	// worker has died, and nothing else would surface that.
	if queueClient != nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				for _, name := range []string{"emails"} {
					for state, depth := range queueClient.Stats(context.Background(), name) {
						telemetry.QueueDepth.WithLabelValues(name, state).Set(float64(depth))
					}
				}
			}
		}()
	}

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

	// AI features. A missing API key yields a disabled client whose
	// calls return ai.ErrDisabled, so the API still boots without one.
	aiClient := ai.NewClient(db)
	if aiClient.Enabled() {
		log.Printf("AI enabled (model: %s)", aiClient.Model())
	} else {
		log.Println("WARNING: no GROQ_API_KEY or OPENAI_API_KEY — AI features disabled")
	}

	recommendationService := &services.RecommendationService{DB: db, Embedder: ai.LexicalEmbedder{}}
	copyGenerator := &services.CopyGeneratorService{
		DB:              db,
		AI:              aiClient,
		Recommendations: recommendationService,
	}
	autoReplyService := &services.AutoReplyService{
		DB:            db,
		AI:            aiClient,
		TicketService: ticketService,
	}
	imageQAService := &services.ImageQAService{DB: db, AI: aiClient}

	// Auto-reply drafter — drafts replies for new tickets every 2 minutes
	if aiClient.Enabled() {
		go func() {
			ticker := time.NewTicker(2 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				drafted, err := autoReplyService.ProcessNewTickets(
					context.Background(), services.DefaultAutoReplyConfig,
				)
				if err != nil {
					log.Printf("auto-reply drafter: %v", err)
					continue
				}
				if drafted > 0 {
					log.Printf("auto-reply drafter: drafted %d ticket(s)", drafted)
				}
			}
		}()
	}

	stripeConnect := &services.StripeConnectService{DB: db}
	bookingService := &services.BookingService{
		DB:            db,
		StripeConnect: stripeConnect,
		EmailService:  emailService,
	}

	graphqlHandler := handler.NewDefaultServer(
		graph.NewExecutableSchema(graph.Config{
			Resolvers: &graph.Resolver{
				DB:             db,
				ProductService: &services.ProductService{DB: db},
				TicketService:  ticketService,
				BookingService: bookingService,
				StripeConnect:  stripeConnect,

				ManufacturingService: manufacturingService,

				AI:              aiClient,
				CopyGenerator:   copyGenerator,
				Recommendations: recommendationService,
				AutoReply:       autoReplyService,
				ImageQA:         imageQAService,
			},
		}),
	)

	// gqlgen recovers resolver panics itself and answers 200 with an
	// errors array, so a panic never reaches chi's Recoverer or the
	// Sentry HTTP middleware. Without this hook, the failures most
	// worth knowing about are the ones that never get reported.
	graphqlHandler.SetRecoverFunc(func(ctx context.Context, err any) error {
		wrapped, ok := err.(error)
		if !ok {
			wrapped = fmt.Errorf("graphql panic: %v", err)
		}

		telemetry.Log.Error("graphql resolver panic",
			zap.String("error", wrapped.Error()),
			zap.String("trace_id", telemetry.TraceIDFromContext(ctx)),
		)
		telemetry.CaptureError(ctx, wrapped, map[string]string{"component": "graphql"})

		// Same opaque message gqlgen's default returns: resolver
		// internals must not travel to the client.
		return errors.New("internal system error")
	})

	if os.Getenv("ENVIRONMENT") != "production" {
		r.Handle("/playground", playground.Handler("GraphQL", "/query"))
	}
	// Order matters: AuthOptional populates the tenant and user in
	// context, the tenant limiter reads the tenant, and the auditor
	// reads both when it records the mutation.
	auditLogger := &appMiddleware.AuditLogger{DB: db}
	r.With(
		appMiddleware.AuthOptional,
		rateLimiter.TenantRateLimit,
		auditLogger.AuditMutations,
	).Handle("/query", graphqlHandler)

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

	telemetry.Log.Info("shutting down gracefully")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Forced shutdown: %v", err)
	}

	// Flush pending spans and a final metric collection after the
	// listener stops but before the process exits, or the last window
	// of telemetry is lost on every deploy.
	shutdownTracing(ctx)
	shutdownMetrics(ctx)

	telemetry.Log.Info("shutdown complete")
}
