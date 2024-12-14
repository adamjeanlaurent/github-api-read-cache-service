package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/adamjeanlaurent/github-api-read-cache-service/cache"
	"github.com/adamjeanlaurent/github-api-read-cache-service/config"
	githubclient "github.com/adamjeanlaurent/github-api-read-cache-service/github-client"
	"github.com/adamjeanlaurent/github-api-read-cache-service/handlers"
	"go.uber.org/zap"
)

// Spawns HTTP Server, and Cache Sync Loop
func StartServer(logger *zap.Logger) error {
	cfg, err := config.NewConfiguration(logger)

	if err != nil {
		return fmt.Errorf("Invalid Configuration: %w", err)
	}

	// Cache Sync Loop and HTTP Server should respect system interupts (e.g CTRL-C)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	githubClient := githubclient.NewGithubClient(cfg, logger)
	dataCache := cache.NewCache(cfg, githubClient, ctx, logger)

	// Start sync loop goroutine for cache
	dataCache.StartSyncLoop()

	httpHandlers := handlers.NewHttpHandlers(cfg, dataCache, logger, githubClient)
	mux := setupApiRoutes(httpHandlers)

	port := fmt.Sprintf(":%d", cfg.GetPort())
	srv := &http.Server{Addr: port, Handler: mux}

	// Graceful server shutdown
	go func() {
		<-ctx.Done()
		logger.Info("Shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("Server forced to shutdown", zap.Error(err))
		}
	}()

	logger.Info("Server is ready to handle requests", zap.String("port", srv.Addr))

	// start server
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Could not start server ", zap.Error(err))
	}

	return nil
}

// Sets up routes for REST API
func setupApiRoutes(httpHandlers handlers.HttpHandlers) *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("GET /healthcheck", httpHandlers.GetHealth())
	mux.Handle("GET /orgs/Netflix", httpHandlers.GetCachedNetflixOrg())
	mux.Handle("GET /orgs/Netflix/members", httpHandlers.GetCachedNetflixOrgMembers())
	mux.Handle("GET /orgs/Netflix/repos", httpHandlers.GetCachedNetflixOrgRepos())
	mux.Handle("GET /view/bottom/{n}/forks", httpHandlers.GetCachedBottomNNetflixReposByForks())
	mux.Handle("GET /view/bottom/{n}/last_updated", httpHandlers.GetCachedBottomNNetflixReposByLastUpdatedTime())
	mux.Handle("GET /view/bottom/{n}/open_issues", httpHandlers.GetCachedBottomNNetflixReposByOpenIssues())
	mux.Handle("GET /view/bottom/{n}/stars", httpHandlers.GetCachedBottomNNetflixReposByStars())

	// catch all, proxies request to github API
	mux.Handle("/", httpHandlers.ProxyRequestToGithubAPI())

	return mux
}
