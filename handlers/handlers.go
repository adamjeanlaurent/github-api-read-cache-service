package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/adamjeanlaurent/github-api-read-cache-service/cache"
	"github.com/adamjeanlaurent/github-api-read-cache-service/config"
	githubclient "github.com/adamjeanlaurent/github-api-read-cache-service/github-client"
	"go.uber.org/zap"
)

type HttpHandlers interface {
	GetHealth() http.Handler
	GetCachedNetflixOrg() http.Handler
	GetCachedNetflixOrgMembers() http.Handler
	GetCachedNetflixOrgRepos() http.Handler
	GetCachedBottomNNetflixReposByForks() http.Handler
	GetCachedBottomNNetflixReposByLastUpdatedTime() http.Handler
	GetCachedBottomNNetflixReposByOpenIssues() http.Handler
	GetCachedBottomNNetflixReposByStars() http.Handler
	ProxyRequestToGithubAPI() http.Handler
}

// Implements the HTTP handlers for service REST API
type httpHandlers struct {
	cfg          config.Configuration
	dataCache    cache.Cache
	logger       *zap.Logger
	githubClient githubclient.GithubClient
}

// Retrieve Newly Created HttpHandlers
func NewHttpHandlers(cfg config.Configuration, dataCache cache.Cache, logger *zap.Logger, githubClient githubclient.GithubClient) HttpHandlers {
	return &httpHandlers{
		cfg:          cfg,
		dataCache:    dataCache,
		logger:       logger,
		githubClient: githubClient,
	}
}

// Responds with Health Status of server
func (handler *httpHandlers) GetHealth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// Responds with cached Netflix Org Data
func (handler *httpHandlers) GetCachedNetflixOrg() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixOrg := handler.dataCache.GetNetflixOrganization()

		w.Header().Set("Content-Type", "application/json")

		if netflixOrg == nil {
			status, err := handler.forceCacheUpdateOnCacheMiss()

			if err != nil {
				http.Error(w, "Error: Cache empty", status)
				return
			}

			netflixOrg = handler.dataCache.GetNetflixOrganization()
		}

		if err := json.NewEncoder(w).Encode(netflixOrg); err != nil {
			handler.logger.Error("Failed to serialize")
			http.Error(w, "Failed to encode json", http.StatusInternalServerError)
		}
	})
}

// Responds with cached list of Netflix Org Members
func (handler *httpHandlers) GetCachedNetflixOrgMembers() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixOrgMembers := handler.dataCache.GetNetflixOrganizationMembers()

		w.Header().Set("Content-Type", "application/json")

		if len(netflixOrgMembers) == 0 {
			status, err := handler.forceCacheUpdateOnCacheMiss()

			if err != nil {
				http.Error(w, "Error: Cache empty", status)
				return
			}

			netflixOrgMembers = handler.dataCache.GetNetflixOrganizationMembers()
		}

		if err := json.NewEncoder(w).Encode(netflixOrgMembers); err != nil {
			handler.logger.Error("Failed to serialize")
			http.Error(w, "Failed to encode json", http.StatusInternalServerError)
		}
	})
}

// Responds with cached list of  Netflix Org Repos
func (handler *httpHandlers) GetCachedNetflixOrgRepos() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetNetflixOrganizationRepos()

		w.Header().Set("Content-Type", "application/json")

		if len(netflixRepos) == 0 {
			status, err := handler.forceCacheUpdateOnCacheMiss()

			if err != nil {
				http.Error(w, "Error: Cache empty", status)
				return
			}

			netflixRepos = handler.dataCache.GetNetflixOrganizationRepos()
		}

		if err := json.NewEncoder(w).Encode(netflixRepos); err != nil {
			handler.logger.Error("Failed to serialize")
			http.Error(w, "Failed to encode json", http.StatusInternalServerError)
		}
	})
}

// Responds with cached Bottom N Netflix Repos By Forks
func (handler *httpHandlers) GetCachedBottomNNetflixReposByForks() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetBottomNetflixReposByForks()

		if len(netflixRepos) == 0 {
			status, err := handler.forceCacheUpdateOnCacheMiss()

			if err != nil {
				http.Error(w, "Error: Cache empty", status)
				return
			}

			netflixRepos = handler.dataCache.GetBottomNetflixReposByForks()
		}

		handler.getBottomNReposHelper(w, r, netflixRepos)
	})
}

// Responds with cached Bottom N Netflix Repos By Last Updated Time
func (handler *httpHandlers) GetCachedBottomNNetflixReposByLastUpdatedTime() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetBottomNetflixReposByUpdateTime()

		if len(netflixRepos) == 0 {
			status, err := handler.forceCacheUpdateOnCacheMiss()

			if err != nil {
				http.Error(w, "Error: Cache empty", status)
				return
			}

			netflixRepos = handler.dataCache.GetBottomNetflixReposByUpdateTime()
		}

		handler.getBottomNReposHelper(w, r, netflixRepos)
	})
}

// Responds with cached Bottom N Netflix Repos By Open Issues
func (handler *httpHandlers) GetCachedBottomNNetflixReposByOpenIssues() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetBottomNetflixReposByOpenIssues()

		if len(netflixRepos) == 0 {
			status, err := handler.forceCacheUpdateOnCacheMiss()

			if err != nil {
				http.Error(w, "Error: Cache empty", status)
				return
			}

			netflixRepos = handler.dataCache.GetBottomNetflixReposByOpenIssues()
		}

		handler.getBottomNReposHelper(w, r, netflixRepos)
	})
}

// Responds with cached Bottom N Netflix Repos By Stars
func (handler *httpHandlers) GetCachedBottomNNetflixReposByStars() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetBottomNetflixReposByStars()

		if len(netflixRepos) == 0 {
			status, err := handler.forceCacheUpdateOnCacheMiss()

			if err != nil {
				http.Error(w, "Error: Cache empty", status)
				return
			}

			netflixRepos = handler.dataCache.GetBottomNetflixReposByStars()
		}

		handler.getBottomNReposHelper(w, r, netflixRepos)
	})
}

// Helper to trim cached bottom view to N length
func (handler *httpHandlers) getBottomNReposHelper(w http.ResponseWriter, r *http.Request, netflixRepos []cache.Tuple) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		http.Error(w, "n must be an integer", http.StatusBadRequest)
		return
	}

	if n <= 0 {
		http.Error(w, "n must be a positive integer", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if n > len(netflixRepos) {
		n = len(netflixRepos)
	}

	if err := json.NewEncoder(w).Encode(netflixRepos[len(netflixRepos)-n:]); err != nil {
		handler.logger.Error("Failed to serialize")
		http.Error(w, "Failed to encode json", http.StatusInternalServerError)
	}
}

// Force Hydrates the cache, to be used on a cache miss
func (handler *httpHandlers) forceCacheUpdateOnCacheMiss() (int, error) {
	handler.logger.Warn("cache miss, forcing cache re-sync", zap.Int("Last sync status", handler.dataCache.GetLastCacheSyncStatus()))

	status, err := handler.dataCache.HydrateCache()

	if err != nil {
		handler.logger.Error("Force cache sync failed", zap.Int("status", status))
	}

	return status, err
}

// Proxies Requests straight to GitHub API.
func (handler *httpHandlers) ProxyRequestToGithubAPI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.githubClient.ForwardRequest(w, r)
	})
}
