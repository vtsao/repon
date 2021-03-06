package repo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-github/v33/github"
	"github.com/gorilla/mux"
	"github.com/vtsao/repon/repo"
)

// fakeGitHubAPIServ creates a fake GitHub REST API server that returns results
// from a hardcoded set of repositories.
func fakeGitHubAPIServ(t *testing.T) *httptest.Server {
	t.Helper()

	router := mux.NewRouter()
	router.HandleFunc("/search/repositories", func(w http.ResponseWriter, r *http.Request) {
		result := &github.RepositoriesSearchResult{
			Total: github.Int(7),
			Repositories: []*github.Repository{
				{
					Name:            github.String("security_monkey"),
					StargazersCount: github.Int(10047),
					ForksCount:      github.Int(792),
					HasIssues:       github.Bool(true),
				},
				{
					Name:            github.String("metaflow"),
					StargazersCount: github.Int(20787),
					ForksCount:      github.Int(2963),
					HasIssues:       github.Bool(true),
				},
				{
					Name:            github.String("SimianArmy"),
					StargazersCount: github.Int(0),
					ForksCount:      github.Int(4253),
					HasIssues:       github.Bool(true),
				},
				{
					Name:            github.String("chaosmonkey"),
					StargazersCount: github.Int(1),
					ForksCount:      github.Int(1017),
					HasIssues:       github.Bool(true),
				},
				{
					Name:            github.String("zuul"),
					StargazersCount: github.Int(0),
					ForksCount:      github.Int(0),
					HasIssues:       github.Bool(true),
				},
				{
					Name:            github.String("Hystrix"),
					StargazersCount: github.Int(10248),
					ForksCount:      github.Int(728),
					HasIssues:       github.Bool(true),
				},
				{
					Name:            github.String("boqboqboq"),
					StargazersCount: github.Int(64),
					ForksCount:      github.Int(9),
					HasIssues:       github.Bool(true),
				},
			},
		}

		r.ParseForm()

		if s := r.Form.Get("sort"); s != "" {
			switch s {
			case "stars":
				sort.Slice(result.Repositories, func(i, j int) bool {
					return *result.Repositories[i].StargazersCount > *result.Repositories[j].StargazersCount
				})
			case "forks":
				sort.Slice(result.Repositories, func(i, j int) bool {
					return *result.Repositories[i].ForksCount > *result.Repositories[j].ForksCount
				})
			}
		}

		// We don't deal with pagination here b/c the TopN tool pages by 100 and we
		// have < 100 fake repos. But ideally a fake should deal with pagination.

		b, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Marshal(%+v) failed: %v", result, err)
		}
		w.Write(b)
	})

	router.HandleFunc("/repos/{owner}/{repo}/pulls", func(w http.ResponseWriter, r *http.Request) {
		prCounts := map[string]int{
			"security_monkey": 55,
			"metaflow":        34555,
			"SimianArmy":      39811,
			"chaosmonkey":     1,
			"zuul":            2305,
			"Hystrix":         0,
			"boqboqboq":       1,
		}

		prCount := prCounts[mux.Vars(r)["repo"]]

		// Set response pagination headers.
		if prCount > 0 {
			// Since the TopN tool always pages by 1 and only looks at the last page
			// to get the total PRs, this is the only page header we need to set.
			q := r.URL.Query()
			q.Set("page", fmt.Sprintf("%d", prCount))
			r.URL.RawQuery = q.Encode()
			w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"last\"", r.URL))
		}

		// Marshal the response.
		var prs []*github.PullRequest
		if prCount > 0 {
			// We only return one object here in this fake b/c the TopN tool always
			// pages by 1 when listing PRs and we can return an empty object b/c the
			// tool also doesn't care about the PR's contents, only the length.
			prs = append(prs, &github.PullRequest{})
		}
		b, err := json.Marshal(prs)
		if err != nil {
			t.Fatalf("Marshal(%+v) failed: %v", prs, err)
		}
		w.Write(b)
	})

	apiHandler := http.NewServeMux()
	apiHandler.Handle("/", router)

	return httptest.NewServer(apiHandler)
}

func TestList(t *testing.T) {
	ctx := context.Background()

	serv := fakeGitHubAPIServ(t)
	client := github.NewClient(nil)
	url, err := url.Parse(serv.URL + "/")
	if err != nil {
		t.Fatalf("Parse(%q) failed: %v", serv.URL+"/", err)
	}
	client.BaseURL = url

	tests := []struct {
		desc               string
		n                  int
		metric             string
		fillPRsConcurrency int
		wantRepos          []*repo.Repo
	}{
		{
			desc:               "top-3 repos by stars",
			n:                  3,
			metric:             "stars",
			fillPRsConcurrency: 1,
			wantRepos: []*repo.Repo{
				{
					Repository: &github.Repository{
						Name:            github.String("metaflow"),
						StargazersCount: github.Int(20787),
						ForksCount:      github.Int(2963),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("Hystrix"),
						StargazersCount: github.Int(10248),
						ForksCount:      github.Int(728),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("security_monkey"),
						StargazersCount: github.Int(10047),
						ForksCount:      github.Int(792),
					},
				},
			},
		},
		{
			desc:               "top-3 repos by forks",
			n:                  3,
			metric:             "forks",
			fillPRsConcurrency: 1,
			wantRepos: []*repo.Repo{
				{
					Repository: &github.Repository{
						Name:            github.String("SimianArmy"),
						StargazersCount: github.Int(0),
						ForksCount:      github.Int(4253),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("metaflow"),
						StargazersCount: github.Int(20787),
						ForksCount:      github.Int(2963),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("chaosmonkey"),
						StargazersCount: github.Int(1),
						ForksCount:      github.Int(1017),
					},
				},
			},
		},
		{
			desc:               "top-3 repos by prs",
			n:                  3,
			metric:             "prs",
			fillPRsConcurrency: 1,
			wantRepos: []*repo.Repo{
				{
					Repository: &github.Repository{
						Name:            github.String("SimianArmy"),
						StargazersCount: github.Int(0),
						ForksCount:      github.Int(4253),
					},
					PRs: 39811,
				},
				{
					Repository: &github.Repository{
						Name:            github.String("metaflow"),
						StargazersCount: github.Int(20787),
						ForksCount:      github.Int(2963),
					},
					PRs: 34555,
				},
				{
					Repository: &github.Repository{
						Name:            github.String("zuul"),
						StargazersCount: github.Int(0),
						ForksCount:      github.Int(0),
					},
					PRs: 2305,
				},
			},
		},
		{
			desc:               "top-3 repos by contribs",
			n:                  3,
			metric:             "contribs",
			fillPRsConcurrency: 1,
			wantRepos: []*repo.Repo{
				{
					Repository: &github.Repository{
						Name:            github.String("metaflow"),
						StargazersCount: github.Int(20787),
						ForksCount:      github.Int(2963),
					},
					PRs: 34555,
				},
				{
					Repository: &github.Repository{
						Name:            github.String("SimianArmy"),
						StargazersCount: github.Int(0),
						ForksCount:      github.Int(4253),
					},
					PRs: 39811,
				},
				{
					Repository: &github.Repository{
						Name:            github.String("boqboqboq"),
						StargazersCount: github.Int(64),
						ForksCount:      github.Int(9),
					},
					PRs: 1,
				},
			},
		},
		{
			desc:               "top-1 repos by stars",
			n:                  1,
			metric:             "stars",
			fillPRsConcurrency: 1,
			wantRepos: []*repo.Repo{
				{
					Repository: &github.Repository{
						Name:            github.String("metaflow"),
						StargazersCount: github.Int(20787),
						ForksCount:      github.Int(2963),
					},
				},
			},
		},
		{
			desc:               "top-9999 repos by stars",
			n:                  9999,
			metric:             "stars",
			fillPRsConcurrency: 1,
			wantRepos: []*repo.Repo{
				{
					Repository: &github.Repository{
						Name:            github.String("metaflow"),
						StargazersCount: github.Int(20787),
						ForksCount:      github.Int(2963),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("Hystrix"),
						StargazersCount: github.Int(10248),
						ForksCount:      github.Int(728),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("security_monkey"),
						StargazersCount: github.Int(10047),
						ForksCount:      github.Int(792),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("boqboqboq"),
						StargazersCount: github.Int(64),
						ForksCount:      github.Int(9),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("chaosmonkey"),
						StargazersCount: github.Int(1),
						ForksCount:      github.Int(1017),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("SimianArmy"),
						StargazersCount: github.Int(0),
						ForksCount:      github.Int(4253),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("zuul"),
						StargazersCount: github.Int(0),
						ForksCount:      github.Int(0),
					},
				},
			},
		},
		{
			desc:               "top-3 repos by stars with concurrency",
			n:                  3,
			metric:             "stars",
			fillPRsConcurrency: 10,
			wantRepos: []*repo.Repo{
				{
					Repository: &github.Repository{
						Name:            github.String("metaflow"),
						StargazersCount: github.Int(20787),
						ForksCount:      github.Int(2963),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("Hystrix"),
						StargazersCount: github.Int(10248),
						ForksCount:      github.Int(728),
					},
				},
				{
					Repository: &github.Repository{
						Name:            github.String("security_monkey"),
						StargazersCount: github.Int(10047),
						ForksCount:      github.Int(792),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			topn := repo.TopN{Client: client, FillPRsConcurrency: tt.fillPRsConcurrency}
			repos, err := topn.List(ctx, "netflix", tt.n, tt.metric)
			if err != nil {
				t.Fatalf(`List("netflix", %d, %q) failed: %v`, tt.n, tt.metric, err)
			}

			if diff := cmp.Diff(tt.wantRepos, repos, cmpopts.IgnoreFields(github.Repository{}, "HasIssues")); diff != "" {
				t.Errorf("List(\"netflix\", %d, %q) got diff (-want +got):\n%s", tt.n, tt.metric, diff)
			}
		})
	}
}
