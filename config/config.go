package config

import (
	"errors"
	"flag"
	"os"
	"time"

	"go.uber.org/zap"
)

type Configuration interface {
	GetGitHubApiKey() string
	GetPort() int
	GetCacheTTL() time.Duration
}

type configuration struct {
	gitHubApiKey string
	port         int
	cacheTTL     time.Duration
}

func (config *configuration) GetGitHubApiKey() string {
	return config.gitHubApiKey
}

func (config *configuration) GetPort() int {
	return config.port
}

func (config *configuration) GetCacheTTL() time.Duration {
	return config.cacheTTL
}

func NewConfiguration(logger *zap.Logger) (Configuration, error) {
	port := flag.Int("port", 0, "Port for server to listen on")

	githubApiKey := os.Getenv("GITHUB_API_TOKEN")
	if len(githubApiKey) == 0 {
		logger.Warn("No GITHUB_API_TOKEN envirnment variable found, may be subject to rate limits")
	}

	if *port == 0 {
		flag.Usage()
		return nil, errors.New("--port is required")
	}

	if *port < 0 || *port > 66535 {
		return nil, errors.New("port must be in valid range (1 to 66535) inclusive")
	}

	// default cache ttl is 10 minutes
	cacheTtl := 10 * time.Minute

	return &configuration{cacheTTL: cacheTtl, port: *port, gitHubApiKey: githubApiKey}, nil
}
