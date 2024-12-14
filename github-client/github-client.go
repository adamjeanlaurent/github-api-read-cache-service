package githubclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type JsonResponse map[string]interface{}

// Client responsible for communicating with Github's REST API. docs: https://docs.github.com/en/rest/quickstart?apiVersion=2022-11-28
type GithubClient interface {
	ForwardRequest(w http.ResponseWriter, r *http.Request, logger *zap.Logger)
	GetNetflixOrg(ctx context.Context) (JsonResponse, error, int)
	GetNetflixOrgMembers(ctx context.Context) ([]JsonResponse, error, int)
	GetNetflixRepos(ctx context.Context) ([]JsonResponse, error, int)
}

type githubClient struct {
	httpClient *http.Client
	apiKey     string
}

// Get newly created GitHubClient
func NewGithubClient(cfg config.Configuration) GithubClient {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &githubClient{
		httpClient: httpClient,
		apiKey:     cfg.GetGitHubApiKey(),
	}
}

// Fetches Netflix Org data
func (ghc *githubClient) GetNetflixOrg(ctx context.Context) (JsonResponse, error, int) {
	return ghc.sendGithubApiRequest(http.MethodGet, ENDPOINT_ORG_NETFLIX, ctx)
}

// Fetches Netflix Org Member data
func (ghc *githubClient) GetNetflixOrgMembers(ctx context.Context) ([]JsonResponse, error, int) {
	return ghc.sendPaginatedGithubApiRequests(http.MethodGet, ENDPOINT_ORG_NETFLIX_MEMBERS, ctx)
}

// Fetches Netflix Org repo data
func (ghc *githubClient) GetNetflixRepos(ctx context.Context) ([]JsonResponse, error, int) {
	return ghc.sendPaginatedGithubApiRequests(http.MethodGet, ENDPOINT_ORG_NETFLIX_REPOS, ctx)
}

// Helper function to make paginated reponses and flatten the responses in a single list
func (ghc *githubClient) sendPaginatedGithubApiRequests(method string, url string, ctx context.Context) ([]JsonResponse, error, int) {
	nextPage := 1
	var flatResponse []JsonResponse

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

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Failed to read response body: %v", err), http.StatusInternalServerError
		}

		var result []JsonResponse
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
func (ghc *githubClient) sendGithubApiRequest(method string, url string, ctx context.Context) (JsonResponse, error, int) {
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request failed"), resp.StatusCode
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response body: %v", err), http.StatusInternalServerError
	}

	var result JsonResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %v", err), http.StatusInternalServerError
	}

	return result, nil, resp.StatusCode
}

// Proxies an incoming http request to the GitHub API
func (ghc *githubClient) ForwardRequest(w http.ResponseWriter, r *http.Request, logger *zap.Logger) {
	targetURL := GITHUB_API_URL + r.URL.Path

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		logger.Error("Failed to create proxy request", zap.Error(err))
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
		logger.Error("Failed to forward proxy request", zap.Error(err))
		http.Error(w, "Failed to forward request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

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
		logger.Error("Failed to copy proxy response body", zap.Error(err))
		http.Error(w, "Failed to copy response body", http.StatusInternalServerError)
	}
}
