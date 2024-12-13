package cache

import (
	"context"
	"fmt"
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
	ttl          time.Duration
	lock         sync.RWMutex
	githubClient githubclient.GithubClient
	ctx          context.Context
	data         *cacheData
	logger       *zap.Logger
}

func NewCache(cfg config.Configuration, client githubclient.GithubClient, context context.Context, logger *zap.Logger) Cache {
	return &cache{ttl: time.Duration(cfg.GetCacheTTL()), githubClient: client, ctx: context}
}

func (c *cache) StartSyncLoop() {
	ticker := time.NewTicker(c.ttl)

	// Try 3 times to initially hydrate the cache
	retriesLeft := 3
	for retriesLeft > 0 {
		c.logger.Info("Hydrating cache for server startup", zap.Int("attempts left", retriesLeft))
		err := c.hydrateCache()
		if err == nil {
			break
		}
		retriesLeft--
	}

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.logger.Info("Attempting to re-Hydrate cache")
				err := c.hydrateCache()

				if err != nil {
					c.logger.Error("Failed to hydrate cache", zap.Error(err))
				} else {
					c.logger.Info("Successfully re-hydrated cache")
				}
			case <-c.ctx.Done():
				c.logger.Info("Cache Ticker Stopped")
				return
			}
		}
	}()
}

func (c *cache) hydrateCache() error {
	// fetch new data
	netflixOrgMembers, err := c.githubClient.GetNetflixOrgMembers(c.ctx)
	if err != nil {
		return fmt.Errorf("Failed to fetch netflix organization members: %s", err.Error())
	}

	netflixOrgRepos, err := c.githubClient.GetNetflixRepos(c.ctx)
	if err != nil {
		return fmt.Errorf("Failed to fetch netflix organization repositories: %s", err.Error())
	}

	netflixOrg, err := c.githubClient.GetNetflixOrg(c.ctx)
	if err != nil {
		return fmt.Errorf("Failed to fetch netflix organization: %s", err.Error())
	}

	// compute views
	var bottomNetflixReposByForks []Tuple
	var bottomNetflixReposByUpdateTime []Tuple
	var bottomNetflixReposByOpenIssues []Tuple
	var bottomNetflixReposByStars []Tuple

	for _, repo := range netflixOrgRepos {
		bottomNetflixReposByForks = append(bottomNetflixReposByForks, Tuple{repo.Name, repo.ForksCount})
		bottomNetflixReposByUpdateTime = append(bottomNetflixReposByUpdateTime, Tuple{repo.Name, repo.UpdatedAt.GetTime().Format(time.RFC3339)})
		bottomNetflixReposByOpenIssues = append(bottomNetflixReposByOpenIssues, Tuple{repo.Name, repo.OpenIssuesCount})
		bottomNetflixReposByStars = append(bottomNetflixReposByStars, Tuple{repo.Name, repo.StargazersCount})
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

	return nil
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
		timeB, _ := time.Parse(time.RFC3339, tuples[a][1].(string))

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
