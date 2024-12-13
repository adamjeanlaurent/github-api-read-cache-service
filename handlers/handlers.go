package handlers

import (
	"encoding/json"
	"fmt"
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

type httpHandlers struct {
	cfg          config.Configuration
	dataCache    cache.Cache
	logger       *zap.Logger
	githubClient githubclient.GithubClient
}

func NewHttpHandlers(cfg config.Configuration, dataCache cache.Cache, logger *zap.Logger, githubClient githubclient.GithubClient) HttpHandlers {
	return &httpHandlers{
		cfg:          cfg,
		dataCache:    dataCache,
		logger:       logger,
		githubClient: githubClient,
	}
}

func (handler *httpHandlers) GetHealth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (handler *httpHandlers) GetCachedNetflixOrg() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixOrg := handler.dataCache.GetNetflixOrganization()

		w.Header().Set("Content-Type", "application/json")

		if netflixOrg == nil {
			http.Error(w, fmt.Sprintf("Previous data sync failed with status code: %d", handler.dataCache.GetLastCacheSyncStatus()), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(netflixOrg); err != nil {
			handler.logger.Error("Failed to serialize")
			http.Error(w, "Failed to encode json", http.StatusInternalServerError)
		}
	})
}

func (handler *httpHandlers) GetCachedNetflixOrgMembers() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixOrgMembers := handler.dataCache.GetNetflixOrganizationMembers()

		w.Header().Set("Content-Type", "application/json")

		if len(netflixOrgMembers) == 0 {
			http.Error(w, fmt.Sprintf("Previous data sync failed with status code: %d", handler.dataCache.GetLastCacheSyncStatus()), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(netflixOrgMembers); err != nil {
			handler.logger.Error("Failed to serialize")
			http.Error(w, "Failed to encode json", http.StatusInternalServerError)
		}
	})
}

func (handler *httpHandlers) GetCachedNetflixOrgRepos() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetNetflixOrganizationRepos()

		w.Header().Set("Content-Type", "application/json")

		if len(netflixRepos) == 0 {
			http.Error(w, fmt.Sprintf("Previous data sync failed with status code: %d", handler.dataCache.GetLastCacheSyncStatus()), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(netflixRepos); err != nil {
			handler.logger.Error("Failed to serialize")
			http.Error(w, "Failed to encode json", http.StatusInternalServerError)
		}
	})
}

func (handler *httpHandlers) GetCachedBottomNNetflixReposByForks() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetBottomNetflixReposByForks()

		handler.getBottomNReposHelper(w, r, netflixRepos)
	})
}

func (handler *httpHandlers) GetCachedBottomNNetflixReposByLastUpdatedTime() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetBottomNetflixReposByUpdateTime()
		handler.getBottomNReposHelper(w, r, netflixRepos)
	})
}

func (handler *httpHandlers) GetCachedBottomNNetflixReposByOpenIssues() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetBottomNetflixReposByOpenIssues()
		handler.getBottomNReposHelper(w, r, netflixRepos)
	})
}

func (handler *httpHandlers) GetCachedBottomNNetflixReposByStars() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		netflixRepos := handler.dataCache.GetBottomNetflixReposByStars()
		handler.getBottomNReposHelper(w, r, netflixRepos)
	})
}

func (handler *httpHandlers) getBottomNReposHelper(w http.ResponseWriter, r *http.Request, netflixRepos []cache.Tuple) {
	if len(netflixRepos) == 0 {
		http.Error(w, fmt.Sprintf("Previous data sync failed with status code: %d. Try again later.", handler.dataCache.GetLastCacheSyncStatus()), http.StatusInternalServerError)
		return
	}

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

	if err := json.NewEncoder(w).Encode(netflixRepos[:n]); err != nil {
		handler.logger.Error("Failed to serialize")
		http.Error(w, "Failed to encode json", http.StatusInternalServerError)
	}
}

func (handler *httpHandlers) ProxyRequestToGithubAPI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.githubClient.ForwardRequest(w, r, handler.logger)
	})
}
