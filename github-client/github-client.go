package githubclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/adamjeanlaurent/github-api-read-cache-service/config"
	"go.uber.org/zap"
)

const (
	GITHUB_API_URL               string = "https://api.github.com"
	ENDPOINT_ORG_NETFLIX         string = GITHUB_API_URL + "/orgs/Netflix"
	ENDPOINT_ORG_NETFLIX_MEMBERS string = GITHUB_API_URL + "/orgs/Netflix/public_members"    // only get public repository members
	ENDPOINT_ORG_NETFLIX_REPOS   string = GITHUB_API_URL + "/orgs/Netflix/repos?type=public" // only get public repositories
	PAGE_SIZE                    int    = 100
)

type JsonObject map[string]interface{}

// Client responsible for communicating with Github's REST API. docs: https://docs.github.com/en/rest/quickstart?apiVersion=2022-11-28
type GithubClient interface {
	ForwardRequest(w http.ResponseWriter, r *http.Request)
	GetNetflixOrg(ctx context.Context) (JsonObject, error, int)
	GetNetflixOrgMembers(ctx context.Context) ([]JsonObject, error, int)
	GetNetflixRepos(ctx context.Context) ([]JsonObject, error, int)
}

type githubClient struct {
	httpClient       *http.Client
	apiKey           string
	inBackoff        bool
	backoffLock      sync.RWMutex
	backoffResetTime time.Time
	logger           *zap.Logger
}

// Get newly created GitHubClient
func NewGithubClient(cfg config.Configuration, logger *zap.Logger) GithubClient {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &githubClient{
		httpClient:       httpClient,
		apiKey:           cfg.GetGitHubApiKey(),
		inBackoff:        false,
		backoffResetTime: time.Now(),
		logger:           logger,
	}
}

// Fetches Netflix Org data
func (ghc *githubClient) GetNetflixOrg(ctx context.Context) (JsonObject, error, int) {
	return ghc.sendGithubApiRequest(http.MethodGet, ENDPOINT_ORG_NETFLIX, ctx)
}

// Fetches Netflix Org Member data
func (ghc *githubClient) GetNetflixOrgMembers(ctx context.Context) ([]JsonObject, error, int) {
	return ghc.sendPaginatedGithubApiRequests(http.MethodGet, ENDPOINT_ORG_NETFLIX_MEMBERS, ctx)
}

// Fetches Netflix Org repo data
func (ghc *githubClient) GetNetflixRepos(ctx context.Context) ([]JsonObject, error, int) {
	return ghc.sendPaginatedGithubApiRequests(http.MethodGet, ENDPOINT_ORG_NETFLIX_REPOS, ctx)
}

// Helper function to make paginated reponses and flatten the responses in a single list
func (ghc *githubClient) sendPaginatedGithubApiRequests(method string, url string, ctx context.Context) ([]JsonObject, error, int) {
	if ghc.shouldBackoff() {
		return nil, fmt.Errorf("Rate Limited, in backoff, try again later"), http.StatusTooManyRequests
	}

	nextPage := 1
	var flatResponse []JsonObject

	for {
		requestUrl := fmt.Sprintf("%s?per_page=%d&page=%d", url, PAGE_SIZE, nextPage)

		req, err := http.NewRequestWithContext(ctx, method, requestUrl, nil)
		if err != nil {
			return nil, fmt.Errorf("Failed to create request: %v", err), http.StatusInternalServerError
		}

		if len(ghc.apiKey) > 0 {
			req.Header.Set("Authorization", "Bearer "+ghc.apiKey)
		}

		resp, err := ghc.httpClient.Do(req)
		if err != nil {
			return nil, err, resp.StatusCode
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Request failed"), resp.StatusCode
		}

		ghc.updateBackoffState(resp.Header)

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Failed to read response body: %v", err), http.StatusInternalServerError
		}

		var result []JsonObject
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("error unmarshalling JSON: %v", err), http.StatusInternalServerError
		}

		if len(result) == 0 {
			break
		}

		flatResponse = append(flatResponse, result...)

		nextPage++
	}

	return flatResponse, nil, http.StatusOK
}

// Helper function to make a non-paginated request
func (ghc *githubClient) sendGithubApiRequest(method string, url string, ctx context.Context) (JsonObject, error, int) {
	if ghc.shouldBackoff() {
		return nil, fmt.Errorf("Rate Limited, in backoff, try again later"), http.StatusTooManyRequests
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create request: %v", err), http.StatusInternalServerError
	}

	if len(ghc.apiKey) > 0 {
		req.Header.Set("Authorization", "Bearer "+ghc.apiKey)
	}

	resp, err := ghc.httpClient.Do(req)
	if err != nil {
		return nil, err, resp.StatusCode
	}

	ghc.updateBackoffState(resp.Header)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request failed"), resp.StatusCode
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response body: %v", err), http.StatusInternalServerError
	}

	var result JsonObject
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %v", err), http.StatusInternalServerError
	}

	return result, nil, resp.StatusCode
}

// Proxies an incoming http request to the GitHub API
func (ghc *githubClient) ForwardRequest(w http.ResponseWriter, r *http.Request) {
	if ghc.shouldBackoff() {
		http.Error(w, "Rate Limited, in backoff, try again later", http.StatusTooManyRequests)
		return
	}

	targetURL := GITHUB_API_URL + r.URL.Path

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		ghc.logger.Error("Failed to create proxy request", zap.Error(err))
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Copy headers from the original request to the new request
	for header, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(header, value)
		}
	}

	gitHubApiKey := ghc.apiKey

	if len(gitHubApiKey) > 0 {
		proxyReq.Header.Set("Authorization", "Bearer "+gitHubApiKey)
	}

	// Send the request to the target service
	resp, err := ghc.httpClient.Do(proxyReq)
	if err != nil {
		ghc.logger.Error("Failed to forward proxy request", zap.Error(err))
		http.Error(w, "Failed to forward request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	ghc.updateBackoffState(resp.Header)

	// Copy response headers to the original response
	for header, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}

	// Write the response status code and body
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		ghc.logger.Error("Failed to copy proxy response body", zap.Error(err))
		http.Error(w, "Failed to copy response body", http.StatusInternalServerError)
	}
}

// determines if the current request should be instantly failed due to backoff, ends backoff if the backoff time period is over
func (ghc *githubClient) shouldBackoff() bool {
	inBackoff := false
	var backoffResetTime time.Time

	ghc.backoffLock.RLock()

	inBackoff = ghc.inBackoff
	backoffResetTime = ghc.backoffResetTime

	ghc.backoffLock.RUnlock()

	if !inBackoff {
		return false
	}

	now := time.Now().UTC()

	if now.After(backoffResetTime) {
		// backoff period over, can start making requests again
		ghc.backoffLock.Lock()

		ghc.inBackoff = false
		ghc.backoffLock.Unlock()

		return false
	} else {
		// backoff period not over yet
		return true
	}
}

// determines it request was rate limited by github, and if so enters backoff for the specified time period
// https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28
func (ghc *githubClient) updateBackoffState(responseHeaders http.Header) {
	// Extract headers
	rateLimitRemaining := responseHeaders.Get("x-ratelimit-remaining")
	rateLimitReset := responseHeaders.Get("x-ratelimit-reset")

	// Parse the x-ratelimit-remaining header
	remaining, err := strconv.Atoi(rateLimitRemaining)
	if err != nil {
		ghc.logger.Error("Error parsing x-ratelimit-remaining", zap.String("remaining", rateLimitRemaining))
		return
	}

	// If x-ratelimit-remaining is 0, github is rate limiting us, enter backoff
	if remaining == 0 {
		resetTime, err := strconv.ParseInt(rateLimitReset, 10, 64)
		if err != nil {
			ghc.logger.Error("Error parsing x-ratelimit-reset", zap.String("reset", rateLimitReset))
			return
		}

		// Convert UTC epoch seconds to time.Time
		resetTimeUTC := time.Unix(resetTime, 0).UTC()

		ghc.logger.Warn("Rate Limited by GitHub API, entering backoff", zap.String("backoff end", resetTimeUTC.String()))

		ghc.backoffLock.Lock()

		ghc.inBackoff = true
		ghc.backoffResetTime = resetTimeUTC

		ghc.backoffLock.Unlock()
	}
}
