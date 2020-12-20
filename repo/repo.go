// Package repo implements finding the top-n GitHub repositories in an
// organization based on different metrics, such as how many stars a repo has.
package repo

import (
	"context"
	"math"
	"sort"

	"github.com/google/go-github/v33/github"
	"golang.org/x/sync/errgroup"
)

type repos []*Repo

func (r repos) Len() int      { return len(r) }
func (r repos) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

// Note that all of these sort in descending order, which is why the Less() is
// reversed.

type byPRs struct{ repos }

func (r byPRs) Less(i, j int) bool { return r.repos[i].PRs > r.repos[j].PRs }

type byContribs struct{ repos }

func (r byContribs) Less(i, j int) bool {
	var contribI float64
	if forksI := *r.repos[i].ForksCount; forksI != 0 {
		contribI = float64(r.repos[i].PRs) / float64(forksI)
	}
	var contribJ float64
	if forksJ := *r.repos[j].ForksCount; forksJ != 0 {
		contribJ = float64(r.repos[j].PRs) / float64(forksJ)
	}
	return contribI > contribJ
}

// Repo holds information about a GitHub repository along with additional data
// about the repo if requested.
type Repo struct {
	*github.Repository
	PRs int
}

// TopN interfaces with the GitHub API to find the top-n GitHub repos in an org
// based on a metric.
type TopN struct {
	Client             *github.Client
	FillPRsConcurrency int
}

// List returns the top-n GitHub repos for the org by metric.
func (t *TopN) List(ctx context.Context, org string, n int, metric string) ([]*Repo, error) {
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	if metric == "stars" || metric == "forks" {
		opts.Sort = metric
		opts.Order = "desc"
	}

	var repos []*Repo
	nextPage := 0
	for {
		opts.ListOptions.Page = nextPage
		result, resp, err := t.Client.Search.Repositories(ctx, "org:"+org, opts)
		if err != nil {
			return nil, err
		}

		for _, r := range result.Repositories {
			repos = append(repos, &Repo{Repository: r})
			// Since Search already sorts stars and forks for us, we can return early
			// here if we've reached n results.
			if (metric == "stars" || metric == "forks") && len(repos) == n {
				return repos, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		nextPage = resp.NextPage
	}

	// Because we can't search repos by PRs using GitHub's repo Search we need to
	// fill in PRs for each repo and sort them to get the top n.
	switch metric {
	case "prs":
		if err := t.fillPRs(ctx, org, repos, metric); err != nil {
			return nil, err
		}
		sort.Sort(byPRs{repos})
	case "contribs":
		if err := t.fillPRs(ctx, org, repos, metric); err != nil {
			return nil, err
		}
		sort.Sort(byContribs{repos})
	}

	n = int(math.Min(float64(n), float64(len(repos))))
	return repos[:n], nil
}

func (t *TopN) fillPRs(ctx context.Context, org string, repos []*Repo, metric string) error {
	for i := 0; i < len(repos); i += t.FillPRsConcurrency {
		end := int(math.Min(float64(i+t.FillPRsConcurrency), float64(len(repos))))
		g, ctx := errgroup.WithContext(ctx)
		for _, repo := range repos[i:end] {
			if !*repo.HasIssues {
				continue
			}
			if metric == "contribs" && *repo.ForksCount == 0 {
				continue
			}

			repo := repo
			g.Go(func() error {
				// Limit to 1 per page so we only need to do one request to count the
				// number of pages to get the total PRs for this repo.
				opts := &github.PullRequestListOptions{
					State:       "all",
					ListOptions: github.ListOptions{PerPage: 1},
				}
				// PullRequests is used instead of Search, since it has a higher quota.
				// We easily run into rate limits when using Search for orgs with lots
				// of repos.
				results, resp, err := t.Client.PullRequests.List(ctx, org, *repo.Name, opts)
				if err != nil {
					return err
				}
				// Number of PRs is just the number of pages unless there is only one
				// page, then it might be 0 or 1.
				repo.PRs = resp.LastPage
				if resp.LastPage == 0 {
					repo.PRs = len(results)
				}

				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}

	return nil
}
