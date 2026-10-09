package main

import (
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/farrasnazhif/ripple-be/docs"
	"github.com/farrasnazhif/ripple-be/internal/database"
	"github.com/farrasnazhif/ripple-be/internal/handler"
	"github.com/farrasnazhif/ripple-be/internal/mailer"
	"github.com/farrasnazhif/ripple-be/internal/openai"
	"github.com/farrasnazhif/ripple-be/internal/paypal"
	"github.com/farrasnazhif/ripple-be/internal/r2"
	"github.com/farrasnazhif/ripple-be/internal/repository"
	"github.com/farrasnazhif/ripple-be/internal/router"
	"github.com/farrasnazhif/ripple-be/internal/service"
)

//	@title			Ripple API
//	@version		1.0
//	@description	User authentication, profile management, and PayPal billing API

//	@host		localhost:8080
//	@BasePath	/api/v1
//	@schemes	http https

//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Bearer token (e.g. "Bearer xxx")

func main() {
	addr := getEnv("ADDR", ":8080")
	jwtSecret := getEnv("JWT_SECRET", "your-secret-key")
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")
	env := getEnv("ENV", "development")

	// Database
	db, err := database.New(database.Config{
		Addr:         getEnv("DB_ADDR", "postgres://admin:adminpassword@localhost/ripple-be?sslmode=disable"),
		MaxOpenConns: 30,
		MaxIdleConns: 30,
		MaxIdleTime:  "15m",
	})
	if err != nil {
		log.Fatal("failed to connect to database:", err)
	}
	defer db.Close()
	log.Println("database connection pool established")

	// Repositories
	userRepo := repository.NewUserRepository(db)
	otpRepo := repository.NewOTPRepository(db)
	billingRepo := repository.NewBillingRepository(db)
	briefRepo := repository.NewBriefRepository(db)
	var briefStorage service.BriefStorage
	if storage, err := r2.New(os.Getenv("R2_ACCOUNT_ID"), os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_SECRET_ACCESS_KEY"), os.Getenv("R2_BUCKET")); err == nil {
		briefStorage = storage
	} else {
		log.Println("R2 attachment storage is not configured")
	}

	// PayPal
	paypalClient := paypal.NewClient(paypal.Config{
		ClientID:  getEnv("PAYPAL_CLIENT_ID", ""),
		Secret:    getEnv("PAYPAL_CLIENT_SECRET", ""),
		BaseURL:   getEnv("PAYPAL_BASE_URL", "https://api-m.sandbox.paypal.com"),
		PlanID:    getEnv("PAYPAL_PRO_PLAN_ID", ""),
		WebhookID: getEnv("PAYPAL_WEBHOOK_ID", ""),
		ReturnURL: getEnv("PAYPAL_RETURN_URL", frontendURL+"/billing/success"),
		CancelURL: getEnv("PAYPAL_CANCEL_URL", frontendURL+"/billing/cancel"),
	})

	// Mailer
	m := mailer.NewResend(getEnv("RESEND_API_KEY", ""), getEnv("FROM_EMAIL", "onboarding@resend.dev"))

	// Services
	authSvc := service.NewAuthService(userRepo, otpRepo, m, jwtSecret, env)
	userSvc := service.NewUserService(userRepo)
	billingSvc := service.NewBillingService(billingRepo, userRepo, paypalClient)
	var briefSummarizer service.BriefSummarizer
	if key, model := os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL"); key != "" && model != "" {
		briefSummarizer = openai.New(key, model)
	}
	briefSvc := service.NewBriefService(briefRepo, briefStorage, briefSummarizer)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc)
	userHandler := handler.NewUserHandler(userSvc)
	billingHandler := handler.NewBillingHandler(billingSvc)
	briefHandler := handler.NewBriefHandler(briefSvc)

	// Router
	mux := router.New(router.Config{
		FrontendURL:     frontendURL,
		JWTSecret:       jwtSecret,
		RateLimit:       30,
		RegisterLimit:   5,
		RateLimitWindow: time.Minute,
	}, authHandler, userHandler, billingHandler, briefHandler, userRepo)

	// Server
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  time.Minute,
	}

	log.Printf("server starting on %s [env=%s]", addr, env)
	log.Fatal(srv.ListenAndServe())
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
