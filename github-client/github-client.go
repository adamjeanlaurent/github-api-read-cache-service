package githubclient

import (
	"context"

	"github.com/adamjeanlaurent/github-api-read-cache-service/config"
	"github.com/google/go-github/v67/github"
)

type GithubClient interface {
	GetNetflixOrg(ctx context.Context) (*github.Organization, error)
	GetNetflixOrgMembers(ctx context.Context) ([]*github.User, error)
	GetNetflixRepos(ctx context.Context) ([]*github.Repository, error)
}

type githubClient struct {
	client *github.Client
}

func (ghc *githubClient) GetNetflixOrg(ctx context.Context) (*github.Organization, error) {
	org, _, err := ghc.client.Organizations.Get(ctx, "Netflix")
	return org, err
}

func (ghc *githubClient) GetNetflixOrgMembers(ctx context.Context) ([]*github.User, error) {
	var allMembers []*github.User

	opts := &github.ListMembersOptions{
		PublicOnly:  true,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		members, resp, err := ghc.client.Organizations.ListMembers(ctx, "Netflix", opts)
		if err != nil {
			return []*github.User{}, err
		}

		allMembers = append(allMembers, members...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage

	}
	return allMembers, nil
}

func (ghc *githubClient) GetNetflixRepos(ctx context.Context) ([]*github.Repository, error) {
	var allRepos []*github.Repository

	opts := &github.RepositoryListByOrgOptions{
		Type:        "public",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := ghc.client.Repositories.ListByOrg(ctx, "Netflix", opts)
		if err != nil {
			return []*github.Repository{}, err
		}

		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}
	return allRepos, nil
}

func NewGithubClient(config config.Configuration) GithubClient {
	client := github.NewClient(nil).WithAuthToken(config.GetGitHubApiKey())
	return &githubClient{client: client}
}
