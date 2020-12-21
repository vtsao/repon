// Package repoql uses the GitHub GraphQL API to implement finding the top-n
// GitHub repositories in an organization based on different metrics, such as
// how many stars a repo has.
package repoql

import (
	"context"
	"math"
	"sort"

	"github.com/shurcooL/githubv4"
)

type repos []*Repo

func (r repos) Len() int      { return len(r) }
func (r repos) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

// Note that all of these sort in descending order, which is why the Less() is
// reversed.

type byStars struct{ repos }

func (r byStars) Less(i, j int) bool {
	return r.repos[i].StargazerCount > r.repos[j].StargazerCount
}

type byForks struct{ repos }

func (r byForks) Less(i, j int) bool {
	return r.repos[i].ForkCount > r.repos[j].ForkCount
}

type byPRs struct{ repos }

func (r byPRs) Less(i, j int) bool {
	return r.repos[i].PullRequests.TotalCount > r.repos[j].PullRequests.TotalCount
}

type byContribs struct{ repos }

func (r byContribs) Less(i, j int) bool {
	var contribI float64
	if forksI := r.repos[i].ForkCount; forksI != 0 {
		contribI = float64(r.repos[i].PullRequests.TotalCount) / float64(forksI)
	}
	var contribJ float64
	if forksJ := r.repos[j].ForkCount; forksJ != 0 {
		contribJ = float64(r.repos[j].PullRequests.TotalCount) / float64(forksJ)
	}
	return contribI > contribJ
}

type PullReq struct {
	TotalCount int
}

type Repo struct {
	Name           string
	StargazerCount int
	ForkCount      int
	PullRequests   *PullReq
}

var q struct {
	Search struct {
		Edges []struct {
			Node struct {
				Repository Repo `graphql:"... on Repository"`
			}
		}
		PageInfo struct {
			EndCursor   githubv4.String
			HasNextPage bool
		}
	} `graphql:"search(query: $query, type: REPOSITORY, first: 100, after: $cursor)"`
}

// TopN interfaces with the GitHub GraphQL API to find the top-n GitHub repos in
// an org based on a metric.
type TopN struct {
	Client *githubv4.Client
}

// List returns the top-n GitHub repos for the org by metric.
func (t *TopN) List(ctx context.Context, org string, n int, metric string) ([]*Repo, error) {
	vars := map[string]interface{}{
		"query":  githubv4.String("org:" + org),
		"cursor": (*githubv4.String)(nil),
	}

	var repos []*Repo
	for {
		err := t.Client.Query(ctx, &q, vars)
		if err != nil {
			return nil, err
		}
		for _, e := range q.Search.Edges {
			r := e.Node.Repository
			repos = append(repos, &r)
		}
		if !q.Search.PageInfo.HasNextPage {
			break
		}
		vars["cursor"] = githubv4.NewString(q.Search.PageInfo.EndCursor)
	}

	// Note that all of these sort in descending order, which is why the less func
	// is reversed.
	switch metric {
	case "stars":
		sort.Sort(byStars{repos})
	case "forks":
		sort.Sort(byForks{repos})
	case "prs":
		sort.Sort(byPRs{repos})
	case "contribs":
		sort.Sort(byContribs{repos})
	}

	n = int(math.Min(float64(n), float64(len(repos))))
	return repos[:n], nil
}
