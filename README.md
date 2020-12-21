# repon

`repon` is a CLI tool that finds the top-n GitHub repositories for an
organization by a metric.

Supported metrics are:

*   Top-n repositories by stars.
*   Top-n repositories by forks.
*   Top-n repositories by pull requests.
*   Top-n repositories by contribution percentage (PRs/forks).

## Getting started

1. `go install -i github.com/vtsao/repon`
1. Authentication is required via a [GitHub Personal Access Token
   (PAT)](https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token)
   with repository privileges.
1. `repon --pat=[YOUR_PAT] --org=netflix --n=5 --metric=prs`

Sample output:

```shell
$ repon --pat=[REDACTED] --org=netflix --n=5 --metric=prs
2020/12/20 22:59:43 Using GitHub GraphQL API
Listing top 5 repos for org "netflix" by "prs"...
1) repo: "lemur", pull requests: 2812
2) repo: "atlas", pull requests: 1037
3) repo: "titus-control-plane", pull requests: 956
4) repo: "conductor", pull requests: 906
5) repo: "genie", pull requests: 889
Took 5.1702134s
```

## GitHub GraphQL API vs. GitHub REST API

`repon` supports using both the [GitHub GraphQL
API](https://docs.github.com/en/free-pro-team@latest/graphql) and [GitHub REST
API](https://docs.github.com/en/free-pro-team@latest/rest). The GraphQL API is
used by default. You can specify using the REST API via `--use_graphql=false`.

### Implementation using GraphQL API

Top-n repositories are listed with the GraphQL by performing a
[Search](https://docs.github.com/en/free-pro-team@latest/graphql/reference/queries#searchresultitemconnection)
query to retrieve all repositories for the organization. The returned
[Repository](https://docs.github.com/en/free-pro-team@latest/graphql/reference/objects#repository)
object contains totals for stars, forks, and pull requests. Then sorting is done
locally and the top-n is returned.

The GraphQL API is preferred over the REST API because all the information for
each repository can be returned in a single API call to calculate the top-n
repositories for a metric.

### Implementation using REST API

When using the REST API, we use the [Search
repositories](https://docs.github.com/en/free-pro-team@latest/rest/reference/search#search-repositories)
method to retrieve all repositories for an organization. When using the `stars`
or `forks` metric, we optimize by having the `Search` method sort for us and we
return early when paging through the `Search` results if we've reached `n`
repositories.

When using `prs` or `contribs` we need to manually join this information for
each repository we retrieve via `Search` because the `Search` results do not
contain pull request information. Then we need to sort all the repositories
locally to return the top-n.

Filling in PR information for a repository is done with the [List pull
requests](https://docs.github.com/en/free-pro-team@latest/rest/reference/pulls#list-pull-requests)
method. In order to avoid paging through all the PRs to find the total, we do a
single list with a `per_page` of `1`, since the pagination results tell us how
many pages there are in total. Because we have to query for total PRs for each
repository, we allow concurrency via `--fill_prs_concurrency`. This should be
set to a small number, otherwise you will trigger GitHub's [Abuse rate
limits](https://docs.github.com/en/free-pro-team@latest/rest/overview/resources-in-the-rest-api#abuse-rate-limits).

> NOTE: finding the total PRs for a repository could have also been done using
> the [Search issues and pull requests](https://docs.github.com/en/free-pro-team@latest/rest/reference/search#search-issues-and-pull-requests)
> method, but `Search` has a significantly lower [rate
> limit](https://docs.github.com/en/free-pro-team@latest/rest/overview/resources-in-the-rest-api#rate-limiting)
> allowance than the core API, which becomes a problem for larger organizations
> containing many repositories.

## Tests

Both `repoql` and `repo` have functional tests against a simplified fake GitHub
GraphQL API and GitHub REST API, respectively. The tests use `httptest` to spin
up a local HTTP server to exercise real network transports.
