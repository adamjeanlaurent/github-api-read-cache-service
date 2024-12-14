package cache

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/adamjeanlaurent/github-api-read-cache-service/config"
	githubclient "github.com/adamjeanlaurent/github-api-read-cache-service/github-client"
	"go.uber.org/zap"
)

type Cache interface {
	StartSyncLoop()
	GetNetflixOrganization() githubclient.JsonObject
	GetNetflixOrganizationMembers() []githubclient.JsonObject
	GetNetflixOrganizationRepos() []githubclient.JsonObject
	GetBottomNetflixReposByForks() []Tuple
	GetBottomNetflixReposByUpdateTime() []Tuple
	GetBottomNetflixReposByOpenIssues() []Tuple
	GetBottomNetflixReposByStars() []Tuple
	GetLastCacheSyncStatus() int
}

type Tuple = [2]interface{}

// Stores In-memory cache of netflix github data, re-hydrates the cache on a fixed interval
type cacheData struct {
	netflixOrganization                githubclient.JsonObject
	netflixOrganizationMembers         []githubclient.JsonObject
	netflixOrganizationRepos           []githubclient.JsonObject
	viewBottomNetflixReposByForks      []Tuple
	viewBottomNetflixReposByUpdateTime []Tuple
	viewBottomNetflixReposByOpenIssues []Tuple
	viewBottomNetflixReposByStars      []Tuple
}

type cache struct {
	ttl                 time.Duration
	lock                sync.RWMutex
	githubClient        githubclient.GithubClient
	ctx                 context.Context
	data                *cacheData
	logger              *zap.Logger
	lastCacheSyncStatus int
}

// Get New Cache
func NewCache(cfg config.Configuration, client githubclient.GithubClient, context context.Context, logger *zap.Logger) Cache {
	return &cache{ttl: time.Duration(cfg.GetCacheTTL()), githubClient: client, ctx: context, logger: logger, lastCacheSyncStatus: http.StatusOK, data: &cacheData{}}
}

// Starts thread that on a fixed interval, makes requests to the GitHub API, computes views, and updates the cache
func (c *cache) StartSyncLoop() {
	ticker := time.NewTicker(c.ttl)

	// Try 5 times to initially hydrate the cache
	retriesLeft := 5
	for retriesLeft > 0 {
		c.logger.Info("Hydrating cache for server startup", zap.Int("attempts left", retriesLeft))

		statusCode, err := c.hydrateCache()
		c.lastCacheSyncStatus = statusCode

		if err == nil {
			c.logger.Info("Successfully hydrated cache")
			break
		}

		c.logger.Warn(fmt.Sprintf("Attempt %d failed backing off for %d seconds", 5-retriesLeft, 5), zap.Error(err), zap.Int("Http status code", statusCode))

		time.Sleep(5 * time.Second)
		retriesLeft--
	}

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.logger.Info("Attempting to re-Hydrate cache")
				statusCode, err := c.hydrateCache()

				if err != nil {
					c.logger.Error("Failed to hydrate cache", zap.Error(err), zap.Int("Http status code", statusCode))
				} else {
					c.logger.Info("Successfully re-hydrated cache")
				}

				c.lastCacheSyncStatus = statusCode
			case <-c.ctx.Done():
				c.logger.Info("Cache Ticker Stopped")
				return
			}
		}
	}()
}

// Makes requests to the GitHub API, computes views, and updates the cache
func (c *cache) hydrateCache() (int, error) {
	// fetch new data
	netflixOrgMembers, err, statusCode := c.githubClient.GetNetflixOrgMembers(c.ctx)
	if err != nil {
		return statusCode, fmt.Errorf("Failed to fetch netflix organization members: %s", err.Error())
	}

	netflixOrgRepos, err, statusCode := c.githubClient.GetNetflixRepos(c.ctx)
	if err != nil {
		return statusCode, fmt.Errorf("Failed to fetch netflix organization repositories: %s", err.Error())
	}

	netflixOrg, err, statusCode := c.githubClient.GetNetflixOrg(c.ctx)
	if err != nil {
		return statusCode, fmt.Errorf("Failed to fetch netflix organization: %s", err.Error())
	}

	// compute views
	var bottomNetflixReposByForks []Tuple
	var bottomNetflixReposByUpdateTime []Tuple
	var bottomNetflixReposByOpenIssues []Tuple
	var bottomNetflixReposByStars []Tuple

	for _, repo := range netflixOrgRepos {
		repoName, ok := repo["name"].(string)
		if !ok {
			return http.StatusInternalServerError, fmt.Errorf("Missing repository name")
		}
		repoName = fmt.Sprintf("Netflix/%s", repoName)

		updatedTime, ok := repo["updated_at"].(string)
		if !ok {
			return http.StatusInternalServerError, fmt.Errorf("Missing Updated time for repository")
		}

		openIssuesCount, ok := repo["open_issues_count"].(float64)
		if !ok {
			return http.StatusInternalServerError, fmt.Errorf("Missing issue count for repository")
		}

		starCount, ok := repo["stargazers_count"].(float64)
		if !ok {
			return http.StatusInternalServerError, fmt.Errorf("Missing star count for repository")
		}

		forksCount, ok := repo["forks_count"].(float64)
		if !ok {
			return http.StatusInternalServerError, fmt.Errorf("Missing forks count for repository")
		}

		bottomNetflixReposByForks = append(bottomNetflixReposByForks, Tuple{repoName, forksCount})
		bottomNetflixReposByUpdateTime = append(bottomNetflixReposByUpdateTime, Tuple{repoName, updatedTime})
		bottomNetflixReposByOpenIssues = append(bottomNetflixReposByOpenIssues, Tuple{repoName, openIssuesCount})
		bottomNetflixReposByStars = append(bottomNetflixReposByStars, Tuple{repoName, starCount})
	}

	sortBottomViewByTimestamp(bottomNetflixReposByUpdateTime)
	sortBottomViewByCount(bottomNetflixReposByForks)
	sortBottomViewByCount(bottomNetflixReposByOpenIssues)
	sortBottomViewByCount(bottomNetflixReposByStars)

	c.lock.Lock()

	c.data = &cacheData{
		netflixOrganization:                netflixOrg,
		netflixOrganizationMembers:         netflixOrgMembers,
		netflixOrganizationRepos:           netflixOrgRepos,
		viewBottomNetflixReposByForks:      bottomNetflixReposByForks,
		viewBottomNetflixReposByStars:      bottomNetflixReposByStars,
		viewBottomNetflixReposByUpdateTime: bottomNetflixReposByUpdateTime,
		viewBottomNetflixReposByOpenIssues: bottomNetflixReposByOpenIssues,
	}

	c.lock.Unlock()

	return http.StatusOK, nil
}

// Sorts list of [name: string, count: float] tuples by count ascending, when count values are the same, uses the name value alphabetically
func sortBottomViewByCount(tuples []Tuple) {
	sort.Slice(tuples, func(a int, b int) bool {
		countA := tuples[a][1].(float64)
		countB := tuples[b][1].(float64)

		if countA == countB {
			nameA := tuples[a][0].(string)
			nameB := tuples[b][0].(string)

			return nameA < nameB
		}

		return countA < countB
	})
}

// Sorts list of [name: string, timestamp: string] tuples by timestamp value ascending
func sortBottomViewByTimestamp(tuples []Tuple) {
	sort.Slice(tuples, func(a int, b int) bool {
		timeA, _ := time.Parse(time.RFC3339, tuples[a][1].(string))
		timeB, _ := time.Parse(time.RFC3339, tuples[b][1].(string))

		return timeA.Before(timeB)
	})
}

// Get Netflix Organization from Cache
func (c *cache) GetNetflixOrganization() githubclient.JsonObject {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.netflixOrganization
}

// Get Netflix Organization Members from Cache
func (c *cache) GetNetflixOrganizationMembers() []githubclient.JsonObject {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.netflixOrganizationMembers
}

// Get Netflix Organization Repos from Cache
func (c *cache) GetNetflixOrganizationRepos() []githubclient.JsonObject {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.netflixOrganizationRepos
}

// Get Bottom Netflix Organization Repos By Forks from Cache
func (c *cache) GetBottomNetflixReposByForks() []Tuple {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.viewBottomNetflixReposByForks
}

// Get Bottom Netflix Organization Repos By Last Updated Time from Cache
func (c *cache) GetBottomNetflixReposByUpdateTime() []Tuple {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.viewBottomNetflixReposByUpdateTime
}

// Get Bottom Netflix Organization Repos By Open Issues from Cache
func (c *cache) GetBottomNetflixReposByOpenIssues() []Tuple {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.viewBottomNetflixReposByOpenIssues
}

// Get Bottom Netflix Organization Repos By Stars from Cache
func (c *cache) GetBottomNetflixReposByStars() []Tuple {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.viewBottomNetflixReposByStars
}

// Get the HTTP status of the last attempted cache sync
func (c *cache) GetLastCacheSyncStatus() int {
	return c.lastCacheSyncStatus
}
