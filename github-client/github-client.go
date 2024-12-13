package githubclient

import (
	"context"
	"net/http"

	"github.com/adamjeanlaurent/github-api-read-cache-service/config"
	"github.com/google/go-github/v67/github"
)

type GithubClient interface {
	GetNetflixOrg(ctx context.Context) (*github.Organization, error, int)
	GetNetflixOrgMembers(ctx context.Context) ([]*github.User, error, int)
	GetNetflixRepos(ctx context.Context) ([]*github.Repository, error, int)
}

type githubClient struct {
	client *github.Client
}

func (ghc *githubClient) GetNetflixOrg(ctx context.Context) (*github.Organization, error, int) {
	org, resp, err := ghc.client.Organizations.Get(ctx, "Netflix")
	return org, err, resp.StatusCode
}

func (ghc *githubClient) GetNetflixOrgMembers(ctx context.Context) ([]*github.User, error, int) {
	var allMembers []*github.User

	opts := &github.ListMembersOptions{
		PublicOnly:  true,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		members, resp, err := ghc.client.Organizations.ListMembers(ctx, "Netflix", opts)
		if err != nil {
			return []*github.User{}, err, resp.StatusCode
		}

		allMembers = append(allMembers, members...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allMembers, nil, http.StatusOK
}

func (ghc *githubClient) GetNetflixRepos(ctx context.Context) ([]*github.Repository, error, int) {
	var allRepos []*github.Repository

	opts := &github.RepositoryListByOrgOptions{
		Type:        "public",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := ghc.client.Repositories.ListByOrg(ctx, "Netflix", opts)
		if err != nil {
			return []*github.Repository{}, err, resp.StatusCode
		}

		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allRepos, nil, http.StatusOK
}

func NewGithubClient(config config.Configuration) GithubClient {
	apiKey := config.GetGitHubApiKey()
	var client *github.Client

	if len(apiKey) > 0 {
		client = github.NewClient(nil).WithAuthToken(config.GetGitHubApiKey())
	} else {
		client = github.NewClient(nil)
	}

	return &githubClient{client: client}
}
