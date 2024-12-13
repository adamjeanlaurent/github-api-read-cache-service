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
	"github.com/google/go-github/v67/github"
	"go.uber.org/zap"
)

type Cache interface {
	StartSyncLoop()
	GetNetflixOrganization() *github.Organization
	GetNetflixOrganizationMembers() []*github.User
	GetNetflixOrganizationRepos() []*github.Repository
	GetBottomNetflixReposByForks() []Tuple
	GetBottomNetflixReposByUpdateTime() []Tuple
	GetBottomNetflixReposByOpenIssues() []Tuple
	GetBottomNetflixReposByStars() []Tuple
	GetLastCacheSyncStatus() int
}

type Tuple = [2]interface{}

type cacheData struct {
	netflixOrganization                *github.Organization
	netflixOrganizationMembers         []*github.User
	netflixOrganizationRepos           []*github.Repository
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

func NewCache(cfg config.Configuration, client githubclient.GithubClient, context context.Context, logger *zap.Logger) Cache {
	return &cache{ttl: time.Duration(cfg.GetCacheTTL()), githubClient: client, ctx: context, logger: logger, lastCacheSyncStatus: http.StatusOK}
}

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
		repoName := fmt.Sprintf("Netflix/%s", *repo.Name)
		bottomNetflixReposByForks = append(bottomNetflixReposByForks, Tuple{repoName, *repo.ForksCount})
		bottomNetflixReposByUpdateTime = append(bottomNetflixReposByUpdateTime, Tuple{repoName, repo.UpdatedAt.GetTime().Format(time.RFC3339)})
		bottomNetflixReposByOpenIssues = append(bottomNetflixReposByOpenIssues, Tuple{repoName, *repo.OpenIssuesCount})
		bottomNetflixReposByStars = append(bottomNetflixReposByStars, Tuple{repoName, *repo.StargazersCount})
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

func sortBottomViewByCount(tuples []Tuple) {
	sort.Slice(tuples, func(a int, b int) bool {
		countA := tuples[a][1].(int)
		countB := tuples[b][1].(int)

		if countA == countB {
			nameA := tuples[a][0].(string)
			nameB := tuples[b][0].(string)

			return nameA < nameB
		}

		return countA < countB
	})
}

func sortBottomViewByTimestamp(tuples []Tuple) {
	sort.Slice(tuples, func(a int, b int) bool {
		timeA, _ := time.Parse(time.RFC3339, tuples[a][1].(string))
		timeB, _ := time.Parse(time.RFC3339, tuples[b][1].(string))

		return timeA.Before(timeB)
	})
}

func (c *cache) GetNetflixOrganization() *github.Organization {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.netflixOrganization
}

func (c *cache) GetNetflixOrganizationMembers() []*github.User {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.netflixOrganizationMembers
}

func (c *cache) GetNetflixOrganizationRepos() []*github.Repository {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.netflixOrganizationRepos
}

func (c *cache) GetBottomNetflixReposByForks() []Tuple {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.viewBottomNetflixReposByForks
}

func (c *cache) GetBottomNetflixReposByUpdateTime() []Tuple {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.viewBottomNetflixReposByUpdateTime
}

func (c *cache) GetBottomNetflixReposByOpenIssues() []Tuple {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.viewBottomNetflixReposByOpenIssues
}

func (c *cache) GetBottomNetflixReposByStars() []Tuple {
	defer c.lock.RUnlock()
	c.lock.RLock()

	return c.data.viewBottomNetflixReposByStars
}

func (c *cache) GetLastCacheSyncStatus() int {
	return c.lastCacheSyncStatus
}
