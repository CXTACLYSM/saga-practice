package http

import (
	"time"

	"github.com/CXTACLYSM/saga-practice/internal"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handlers *internal.Handlers) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.Heartbeat("/ping"))

	r.Group(func(r chi.Router) {
		r.Post("/api/v1/transfers", handlers.CreateHandler.ServeHTTP)
	})

	return r
}
