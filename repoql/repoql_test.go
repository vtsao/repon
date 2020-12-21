package repoql_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/mux"
	"github.com/shurcooL/githubv4"
	"github.com/vtsao/repon/repoql"
)

// fakeGitHubAPIServ creates a fake GitHub GraphQL API server that returns
// results from a hardcoded set of repositories.
func fakeGitHubAPIServ(t *testing.T) *httptest.Server {
	t.Helper()

	router := mux.NewRouter()
	router.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		result := `{
  "data": {
    "search": {
      "edges": [
        {
          "node": {
            "name": "security_monkey",
            "stargazerCount": 10047,
            "forkCount": 792,
            "pullRequests": {
              "totalCount": 55
            }
          }
        },
        {
          "node": {
            "name": "metaflow",
            "stargazerCount": 20787,
            "forkCount": 2963,
            "pullRequests": {
              "totalCount": 34555
            }
          }
        },
        {
          "node": {
            "name": "SimianArmy",
            "stargazerCount": 0,
            "forkCount": 4253,
            "pullRequests": {
              "totalCount": 39811
            }
          }
        },
        {
          "node": {
            "name": "chaosmonkey",
            "stargazerCount": 1,
            "forkCount": 1017,
            "pullRequests": {
              "totalCount": 1
            }
          }
        },
        {
          "node": {
            "name": "zuul",
            "stargazerCount": 0,
            "forkCount": 0,
            "pullRequests": {
              "totalCount": 2305
            }
          }
        },
        {
          "node": {
            "name": "Hystrix",
            "stargazerCount": 10248,
            "forkCount": 728,
            "pullRequests": {
              "totalCount": 0
            }
          }
        },
        {
          "node": {
            "name": "boqboqboq",
            "stargazerCount": 64,
            "forkCount": 9,
            "pullRequests": {
              "totalCount": 1
            }
          }
        }
      ]
    }
  }
}`

		// We don't deal with pagination here b/c the TopN tool pages by 100 and we
		// have < 100 fake repos. But ideally a fake should deal with pagination.

		if _, err := io.WriteString(w, result); err != nil {
			t.Fatalf("WriteString(%s) failed: %v", result, err)
		}
	})

	apiHandler := http.NewServeMux()
	apiHandler.Handle("/", router)

	return httptest.NewServer(apiHandler)
}

func TestList(t *testing.T) {
	ctx := context.Background()

	serv := fakeGitHubAPIServ(t)
	client := githubv4.NewEnterpriseClient(serv.URL+"/graphql", nil)
	topn := repoql.TopN{Client: client}

	tests := []struct {
		desc      string
		n         int
		metric    string
		wantRepos []*repoql.Repo
	}{
		{
			desc:   "top-3 repos by stars",
			n:      3,
			metric: "stars",
			wantRepos: []*repoql.Repo{
				{
					Name:           "metaflow",
					StargazerCount: 20787,
					ForkCount:      2963,
					PullRequests:   &repoql.PullReq{TotalCount: 34555},
				},
				{
					Name:           "Hystrix",
					StargazerCount: 10248,
					ForkCount:      728,
					PullRequests:   &repoql.PullReq{TotalCount: 0},
				},
				{
					Name:           "security_monkey",
					StargazerCount: 10047,
					ForkCount:      792,
					PullRequests:   &repoql.PullReq{TotalCount: 55},
				},
			},
		},
		{
			desc:   "top-3 repos by forks",
			n:      3,
			metric: "forks",
			wantRepos: []*repoql.Repo{
				{
					Name:           "SimianArmy",
					StargazerCount: 0,
					ForkCount:      4253,
					PullRequests:   &repoql.PullReq{TotalCount: 39811},
				},
				{
					Name:           "metaflow",
					StargazerCount: 20787,
					ForkCount:      2963,
					PullRequests:   &repoql.PullReq{TotalCount: 34555},
				},
				{
					Name:           "chaosmonkey",
					StargazerCount: 1,
					ForkCount:      1017,
					PullRequests:   &repoql.PullReq{TotalCount: 1},
				},
			},
		},
		{
			desc:   "top-3 repos by prs",
			n:      3,
			metric: "prs",
			wantRepos: []*repoql.Repo{
				{
					Name:           "SimianArmy",
					StargazerCount: 0,
					ForkCount:      4253,
					PullRequests:   &repoql.PullReq{TotalCount: 39811},
				},
				{
					Name:           "metaflow",
					StargazerCount: 20787,
					ForkCount:      2963,
					PullRequests:   &repoql.PullReq{TotalCount: 34555},
				},
				{
					Name:           "zuul",
					StargazerCount: 0,
					ForkCount:      0,
					PullRequests:   &repoql.PullReq{TotalCount: 2305},
				},
			},
		},
		{
			desc:   "top-3 repos by contribs",
			n:      3,
			metric: "contribs",
			wantRepos: []*repoql.Repo{
				{
					Name:           "metaflow",
					StargazerCount: 20787,
					ForkCount:      2963,
					PullRequests:   &repoql.PullReq{TotalCount: 34555},
				},
				{
					Name:           "SimianArmy",
					StargazerCount: 0,
					ForkCount:      4253,
					PullRequests:   &repoql.PullReq{TotalCount: 39811},
				},
				{
					Name:           "boqboqboq",
					StargazerCount: 64,
					ForkCount:      9,
					PullRequests:   &repoql.PullReq{TotalCount: 1},
				},
			},
		},
		{
			desc:   "top-1 repos by stars",
			n:      1,
			metric: "stars",
			wantRepos: []*repoql.Repo{
				{
					Name:           "metaflow",
					StargazerCount: 20787,
					ForkCount:      2963,
					PullRequests:   &repoql.PullReq{TotalCount: 34555},
				},
			},
		},
		{
			desc:   "top-9999 repos by stars",
			n:      9999,
			metric: "stars",
			wantRepos: []*repoql.Repo{
				{
					Name:           "metaflow",
					StargazerCount: 20787,
					ForkCount:      2963,
					PullRequests:   &repoql.PullReq{TotalCount: 34555},
				},
				{
					Name:           "Hystrix",
					StargazerCount: 10248,
					ForkCount:      728,
					PullRequests:   &repoql.PullReq{TotalCount: 0},
				},
				{
					Name:           "security_monkey",
					StargazerCount: 10047,
					ForkCount:      792,
					PullRequests:   &repoql.PullReq{TotalCount: 55},
				},
				{
					Name:           "boqboqboq",
					StargazerCount: 64,
					ForkCount:      9,
					PullRequests:   &repoql.PullReq{TotalCount: 1},
				},
				{
					Name:           "chaosmonkey",
					StargazerCount: 1,
					ForkCount:      1017,
					PullRequests:   &repoql.PullReq{TotalCount: 1},
				},
				{
					Name:           "SimianArmy",
					StargazerCount: 0,
					ForkCount:      4253,
					PullRequests:   &repoql.PullReq{TotalCount: 39811},
				},
				{
					Name:           "zuul",
					StargazerCount: 0,
					ForkCount:      0,
					PullRequests:   &repoql.PullReq{TotalCount: 2305},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			repos, err := topn.List(ctx, "netflix", tt.n, tt.metric)
			if err != nil {
				t.Fatalf(`List("netflix", %d, %q) failed: %v`, tt.n, tt.metric, err)
			}

			if diff := cmp.Diff(tt.wantRepos, repos); diff != "" {
				t.Errorf("List(\"netflix\", %d, %q) got diff (-want +got):\n%s", tt.n, tt.metric, diff)
			}
		})
	}
}
