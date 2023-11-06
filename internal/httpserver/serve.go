package httpserver

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hashicorp/hcl/v2"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"

	"github.com/hiphops-io/hops/dsl"
	"github.com/hiphops-io/hops/logs"
)

type NatsClient interface {
	Publish(context.Context, []byte, ...string) (*jetstream.PubAck, error, bool)
	CheckConnection() bool
}

func Serve(addr string, hops hcl.Body, natsClient NatsClient, logger zerolog.Logger) error {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RedirectSlashes)
	r.Use(logs.AccessLogMiddleware(logger))
	r.Use(Healthcheck(natsClient, "/health"))
	// TODO: Make CORS configurable and lock down by default. As-is it would be
	// insecure for production/deployed use.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Serve the single page app for the console from the UI dir
	r.Mount("/console", ConsoleRouter(logger))

	// Serve the tasks API
	taskHops, err := parseTasks(hops)
	if err != nil {
		return err
	}

	r.Mount("/tasks", TaskRouter(taskHops, natsClient, logger))

	logger.Info().Msgf("Console available on http://%s/console", addr)
	return http.ListenAndServe(addr, r)
}

func parseTasks(hops hcl.Body) (*dsl.HopAST, error) {
	ctx := context.Background()
	taskHops, err := dsl.ParseHopsTasks(ctx, hops)
	if err != nil {
		return nil, err
	}

	return taskHops, nil
}
