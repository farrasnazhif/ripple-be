package router

import (
	"net/http"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/handler"
	mw "github.com/farrasnazhif/ripple-be/internal/middleware"
	"github.com/farrasnazhif/ripple-be/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type Config struct {
	FrontendURL     string
	JWTSecret       string
	RateLimit       int
	RegisterLimit   int
	RateLimitWindow time.Duration
}

func New(cfg Config, authHandler *handler.AuthHandler, userHandler *handler.UserHandler, billingHandler *handler.BillingHandler, briefHandler *handler.BriefHandler, userRepo repository.UserRepository) http.Handler {
	r := chi.NewRouter()

	r.Use(mw.CORS(cfg.FrontendURL))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	registerLimiter := mw.RateLimit(cfg.RegisterLimit, cfg.RateLimitWindow)
	apiLimiter := mw.RateLimit(cfg.RateLimit, cfg.RateLimitWindow)
	auth := mw.Auth(cfg.JWTSecret, userRepo)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		})

		r.Get("/swagger/*", httpSwagger.WrapHandler)
		r.Get("/demo/graph", handler.DemoGraph)
		r.With(apiLimiter).Post("/impact/preview", handler.PreviewImpact)

		r.Route("/auth", func(r chi.Router) {
			r.With(registerLimiter).Post("/register", authHandler.Register)
			r.With(registerLimiter).Post("/otp/send", authHandler.SendOTP)
			r.With(registerLimiter).Post("/otp/verify", authHandler.VerifyOTP)
			r.With(apiLimiter).Post("/login", authHandler.Login)
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(auth)
			r.Get("/me", userHandler.GetProfile)
			r.Get("/billing", userHandler.GetBilling)
		})

		r.Route("/briefs", func(r chi.Router) {
			r.Use(auth)
			r.Use(apiLimiter)
			r.Post("/", briefHandler.Create)
			r.Get("/{briefID}", briefHandler.Get)
			r.Post("/{briefID}/summary/draft", briefHandler.DraftSummary)
			r.Put("/{briefID}/summary", briefHandler.ConfirmSummary)
			r.Post("/{briefID}/attachments", briefHandler.StartAttachment)
			r.Get("/{briefID}/attachments", briefHandler.ListAttachments)
			r.Post("/{briefID}/attachments/{attachmentID}/complete", briefHandler.CompleteAttachment)
			r.Get("/{briefID}/attachments/{attachmentID}/download", briefHandler.DownloadAttachment)
		})

		// Webhook - no auth required
		r.With(apiLimiter).Post("/billing/paypal/webhook", billingHandler.Webhook)

		// Billing - authenticated
		r.Route("/billing", func(r chi.Router) {
			r.Use(auth)
			r.Use(apiLimiter)
			r.Post("/paypal/subscriptions", billingHandler.CreateSubscription)
			r.Delete("/paypal/subscriptions", billingHandler.CancelSubscription)
		})
	})

	return r
}
