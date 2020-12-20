// Binary repo-n lists the top-n GitHub repositories for an organization based
// on a metric.
//
// Usage:
//   go build -o repo-n main.go
//   ./repo-n --org=netflix --n=10 --metric=stars
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/go-github/v33/github"
	"github.com/vtsao/repon/repo"
)

var (
	org    = flag.String("org", "", "required, the organization to get repos for")
	n      = flag.Int("n", 0, "required, the top n repos to get")
	metric = flag.String("metric", "stars", `the metric to sort repos by, must be one of ["stars", "forks", "prs", "contribs"]`)

	// Specify these if running into rate limit issues. An OAuth app can be
	// created at https://github.com/settings/developers.
	clientID     = flag.String("client_id", "", "OAuth2 client ID for higher rate limits")
	clientSecret = flag.String("client_secret", "", "OAuth2 client secret for higher rate limits")

	fillPRsConcurrency = flag.Int("fill_prs_concurrency", 10, `number of concurrent calls to GitHub Issues API to count PRs per repo if using one of ["prs", "contribs"]`)
)

func main() {
	start := time.Now()

	flag.Parse()
	ctx := context.Background()

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
	if (*clientID == "") != (*clientSecret == "") {
		flag.PrintDefaults()
		log.Fatal("Either none or both of --client_id and --client_secret must be specified")
	}

	var httpClient *http.Client
	if *clientID != "" && *clientSecret != "" {
		t := &github.UnauthenticatedRateLimitedTransport{
			ClientID:     *clientID,
			ClientSecret: *clientSecret,
		}
		httpClient = t.Client()
	}
	client := github.NewClient(httpClient)
	topn := repo.TopN{Client: client, FillPRsConcurrency: *fillPRsConcurrency}

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

	fmt.Printf("Took %s\n", time.Since(start))
}
