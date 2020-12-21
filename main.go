// Binary repo-n lists the top-n GitHub repositories for an organization based
// on a metric.
//
// Usage:
//   go build -o repo-n main.go
//   ./repo-n --pat=[YOUR_PAT] --org=netflix --n=10 --metric=stars
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/go-github/v33/github"
	"github.com/shurcooL/githubv4"
	"github.com/vtsao/repon/repo"
	"github.com/vtsao/repon/repoql"
	"golang.org/x/oauth2"
)

var (
	org    = flag.String("org", "", "required, the organization to get repos for")
	n      = flag.Int("n", 0, "required, the top n repos to get")
	metric = flag.String("metric", "stars", `the metric to sort repos by, must be one of ["stars", "forks", "prs", "contribs"]`)

	// See https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token
	// for how to create one.
	pat = flag.String("pat", "", "required, GitHub OAuth2 personal access token")

	useGraphQL = flag.Bool("use_graphql", true, "whether to use GitHub's GraphQL API or the REST API")

	fillPRsConcurrency = flag.Int("fill_prs_concurrency", 10, `number of concurrent calls to GitHub Issues REST API to count PRs per repo if using one of ["prs", "contribs"]; only applicable if --use_graph_ql=false`)
)

func validateFlags() {
	if *org == "" {
		flag.PrintDefaults()
		log.Fatal("--org is required")
	}
	if *n == 0 {
		flag.PrintDefaults()
		log.Fatal("--n is required")
	}
	if m := *metric; m != "stars" && m != "forks" && m != "prs" && m != "contribs" {
		flag.PrintDefaults()
		log.Fatal(`--metric must be one of ["stars", "forks", "prs", "contribs"]`)
	}
	if *pat == "" {
		flag.PrintDefaults()
		log.Fatal("--pat is required")
	}
}

func repon(ctx context.Context, client *http.Client) {
	topn := repo.TopN{
		Client:             github.NewClient(client),
		FillPRsConcurrency: *fillPRsConcurrency,
	}

	fmt.Printf("Listing top %d repos for org %q by %q...\n", *n, *org, *metric)
	repos, err := topn.List(ctx, *org, *n, *metric)
	if err != nil {
		log.Fatalf("Error listing top %d repos for org %q by %q: %v", *n, *org, *metric, err)
	}

	for i, r := range repos {
		switch *metric {
		case "stars":
			fmt.Printf("%d) repo: %q, stars: %d\n", i+1, *r.Name, *r.StargazersCount)
		case "forks":
			fmt.Printf("%d) repo: %q, forks: %d\n", i+1, *r.Name, *r.ForksCount)
		case "prs":
			fmt.Printf("%d) repo: %q, pull requests: %d\n", i+1, *r.Name, r.PRs)
		case "contribs":
			var contrib float64
			if forks := *r.ForksCount; forks > 0 {
				contrib = float64(r.PRs) / float64(forks)
			}
			fmt.Printf("%d) repo: %q, contribution percentage: %.2f%%\n", i+1, *r.Name, contrib*100)
		}
	}
}

func reponQL(ctx context.Context, client *http.Client) {
	topn := repoql.TopN{Client: githubv4.NewClient(client)}

	fmt.Printf("Listing top %d repos for org %q by %q...\n", *n, *org, *metric)
	repos, err := topn.List(ctx, *org, *n, *metric)
	if err != nil {
		log.Fatalf("Error listing top %d repos for org %q by %q: %v", *n, *org, *metric, err)
	}

	for i, r := range repos {
		switch *metric {
		case "stars":
			fmt.Printf("%d) repo: %q, stars: %d\n", i+1, r.Name, r.StargazerCount)
		case "forks":
			fmt.Printf("%d) repo: %q, forks: %d\n", i+1, r.Name, r.ForkCount)
		case "prs":
			fmt.Printf("%d) repo: %q, pull requests: %d\n", i+1, r.Name, r.PullRequests.TotalCount)
		case "contribs":
			var contrib float64
			if forks := r.ForkCount; forks > 0 {
				contrib = float64(r.PullRequests.TotalCount) / float64(forks)
			}
			fmt.Printf("%d) repo: %q, contribution percentage: %.2f%%\n", i+1, r.Name, contrib*100)
		}
	}
}

func main() {
	start := time.Now()
	ctx := context.Background()
	flag.Parse()
	validateFlags()
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *pat}))

	if *useGraphQL {
		log.Print("Using GitHub GraphQL API")
		reponQL(ctx, client)
	} else {
		log.Print("Using GitHub REST API")
		repon(ctx, client)
	}

	fmt.Printf("Took %s\n", time.Since(start))
}
