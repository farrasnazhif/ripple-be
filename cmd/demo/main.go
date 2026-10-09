package main

import (
	"log"
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/handler"
	"github.com/farrasnazhif/ripple-be/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// A database-free server for the clearly labeled synthetic graph demo.
func main() {
	r := chi.NewRouter()
	r.Use(middleware.CORS("http://localhost:3000"))
	r.Get("/api/v1/demo/graph", handler.DemoGraph)
	r.Post("/api/v1/impact/preview", handler.PreviewImpact)
	log.Fatal(http.ListenAndServe(":8081", r))
}
