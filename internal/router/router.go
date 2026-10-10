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
	Budget          *handler.BudgetHandler
	Review          *handler.ReviewHandler
	Workflow        *handler.WorkflowHandler
	FrontendURL     string
	JWTSecret       string
	RateLimit       int
	RegisterLimit   int
	RateLimitWindow time.Duration
}

func New(cfg Config, authHandler *handler.AuthHandler, userHandler *handler.UserHandler, projectHandler *handler.ProjectHandler, briefHandler *handler.BriefHandler, userRepo repository.UserRepository) http.Handler {
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
		r.With(auth).Get("/demo/graph", handler.DemoGraph)
		r.With(auth, apiLimiter).Post("/impact/preview", handler.PreviewImpact)

		r.Route("/auth", func(r chi.Router) {
			r.With(registerLimiter).Post("/register", authHandler.Register)
			r.With(apiLimiter).Post("/login", authHandler.Login)
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(auth)
			r.Get("/me", userHandler.GetProfile)
		})

		r.With(auth, apiLimiter).Get("/projects", projectHandler.List)
		r.With(auth, apiLimiter).Get("/projects/", projectHandler.List)
		r.With(auth, apiLimiter).Post("/projects", projectHandler.Create)
		r.With(auth, apiLimiter).Post("/projects/", projectHandler.Create)
		r.With(auth, apiLimiter).Get("/projects/{projectID}", projectHandler.Get)
		r.With(auth, apiLimiter).Patch("/projects/{projectID}", projectHandler.Update)
		r.With(auth, apiLimiter).Delete("/projects/{projectID}", projectHandler.Delete)

		if cfg.Budget != nil {
			r.With(auth).Get("/projects/{projectID}/budget", cfg.Budget.Get)
			r.With(auth, apiLimiter).Put("/projects/{projectID}/budget", cfg.Budget.Set)
		}
		if cfg.Workflow != nil {
			r.With(auth).Get("/projects/{projectID}/storyboard", cfg.Workflow.ListShots)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/storyboard", cfg.Workflow.CreateShots)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/shots/{shotID}/generations", cfg.Workflow.Submit)
			r.With(auth).Get("/projects/{projectID}/generations", cfg.Workflow.ListJobs)
			r.With(auth).Get("/projects/{projectID}/generations/{jobID}", cfg.Workflow.GetJob)
		}

		if cfg.Review != nil {
			r.With(auth).Get("/projects/{projectID}/review", cfg.Review.Get)
			r.With(auth, apiLimiter).Put("/projects/{projectID}/brief", cfg.Review.Edit)
			r.With(auth, apiLimiter).Put("/projects/{projectID}/output-type", cfg.Review.SetType)
			r.With(auth, apiLimiter).Put("/projects/{projectID}/shots/{shotID}/requirements", cfg.Review.Link)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/shots/{shotID}/decisions", cfg.Review.Decide)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/plan/draft", cfg.Review.Draft)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/summary/refine", cfg.Review.Draft)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/images", cfg.Review.GenerateImage)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/final", cfg.Review.Final)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/finals/{finalID}/refinements", cfg.Review.RefineFinal)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/finals/{finalID}/regenerations", cfg.Review.RegenerateFinal)
			r.With(auth, apiLimiter).Get("/projects/{projectID}/finals/{finalID}", cfg.Review.GetFinal)
			r.With(auth, apiLimiter).Post("/projects/{projectID}/finals/{finalID}/accept", cfg.Review.AcceptFinal)
		}

		r.Route("/briefs", func(r chi.Router) {
			r.Use(auth)
			r.Use(apiLimiter)
			r.Post("/", briefHandler.Create)
			r.Get("/storage", briefHandler.StorageStatus)
			r.Get("/{briefID}", briefHandler.Get)
			r.Put("/{briefID}/text", briefHandler.SetText)
			r.Post("/{briefID}/summary/draft", briefHandler.DraftSummary)
			r.Put("/{briefID}/summary", briefHandler.ConfirmSummary)
			r.Post("/{briefID}/attachments", briefHandler.StartAttachment)
			r.Get("/{briefID}/attachments", briefHandler.ListAttachments)
			r.Post("/{briefID}/attachments/{attachmentID}/complete", briefHandler.CompleteAttachment)
			r.Get("/{briefID}/attachments/{attachmentID}/download", briefHandler.DownloadAttachment)
		})

	})

	return r
}
