package config

import (
	"errors"
	"os"
	"strconv"
	"time"
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

// expected args: 1: api key, 2: port 3: scrape interval
func NewConfiguration() (Configuration, error) {
	if len(os.Args) < 3 {
		return nil, errors.New("Expected Args = required:{github api key} required:{port} optional:{cacheTTL}")
	}

	githubApiKey := os.Args[1]

	port, err := strconv.Atoi(os.Args[2])

	if err != nil {
		return nil, errors.New("port must be a valid integer")
	}

	if port < 0 || port > 66535 {
		return nil, errors.New("port must be in valid range (1 to 66535) inclusive")
	}

	// default cache ttl is 10 minutes
	cacheTtl := 10 * time.Minute

	return &configuration{cacheTTL: cacheTtl, port: port, gitHubApiKey: githubApiKey}, nil
}
