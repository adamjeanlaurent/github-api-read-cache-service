# Netflix Github API Read Cache Service

## Running Locally

### Clone the Repository
```git clone https://github.com/adamjeanlaurent/github-api-read-cache-service.git```

```cd github-api-read-cache-service```

### Building
Golang is required: https://go.dev/doc/install

Run the build script to build the service in all flavors.
```./scripts/build.sh```

### Running
After the build is finished the following binaries are built. ```(server-linux-amd, server-linux-arm, server-mac-amd, server-mac-arm, server-windows-amd.exe, server-windows-arm.exe)```

Run the appropriate binary, port (the port the server runs on) is a required argument. Optionally set an environment variable GITHUB_API_TOKEN with a github API token.

ex. ```./bin/server-mac-arm --port=7101```

ex. ```GITHUB_API_TOKEN=xyz123 ./bin/server-mac-arm --port=7101```

### Testing

Make requests to any of the following endpoints

```
http://localhost:{PORT}/healthcheck
http://localhost:{PORT}/orgs/Netflix
http://localhost:{PORT}/orgs/Netflix/members
http://localhost:{PORT}/orgs/Netflix/repos
http://localhost:{PORT}/view/bottom/{n}/forks
http://localhost:{PORT/view/bottom/{n}/last_updated
http://localhost:{PORT}/view/bottom/{n}/open_issues
http://localhost:{PORT}/view/bottom/{n}/stars
Any Other GitHub REST API Endpont (https://docs.github.com/en/rest?apiVersion=2022-11-28)
```

# Design Decisions

![image](https://github.com/user-attachments/assets/a999bf1f-76a7-4d61-b055-33fd706486c7)


## Dedicated Thread for Cache Warming 
See [cache.StartSyncLoop()](https://github.com/adamjeanlaurent/github-api-read-cache-service/blob/main/cache/cache.go#L58).

The server has a long-living dedicated thread that every 10 minutes (and at server startup), warms the cache via fetching /orgs/Netflix, /orgs/Netflix/members/, /orgs/Netflix/repos from the GitHub API, flattening the reponeses, and computing bottom repo views by forks, issues, update time, and stars.

The fetched and computed data is cached in memory, to be served when users ask for it.

I chose cache warming for a few reasons. 

1. Lowers client latency to our service, as no fetch requests to the GitHub API need to happen at client request time, the cached data will always be available in-memory. 
2. Data returned by the service is always consistent within 10 minutes of real data from GitHub. 10 Minutes is sort of an arbitrary TTL, it could be longer if we're okay with returning staler data.
3. Limits the amount of calls we make to the actual github API (outside of Proxy requests) to a only a couple every 10 minutes, meaning we won't cause burden to github with constant flurries of requests.

## Forced Cache Fetch on Cache Miss

see [httpHandlers.forceCacheUpdateOnCacheMiss()](https://github.com/adamjeanlaurent/github-api-read-cache-service/blob/main/handlers/handlers.go#L231)

The server pre-warms the cache during start up, however, if all those requests fail (it retires 5 times), then all later requests for cached data would fail until the next successful cache sync loop iteration in 10 minutres. That would mean 10 minutes of downtime for some routes.

To combat this, if a request comes in for cached data, and for some reason, the cache it empty, the data will be force synced and stored in the cache.

This stops there from being downtime for cached requests in the time between failed cache sync loop updates. Lowering downtimes for users.

## Backoff 

See [githubClient.updateBackoffState()](https://github.com/adamjeanlaurent/github-api-read-cache-service/blob/main/github-client/github-client.go#L258).

The GitHub API has rate limits https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28.

When the GitHub API fails a request because we've exceeded our rate limit, the service enters 'backoff'. The service will not attempt to make any requests to GitHub until the rate limits resets. All cached requests to the service will continue to work.

The GitHub API may entierly block your IP from making requests or increase the rate limit period if you keep sending requests that are rate limited, so having backoff will stop us from spamming GitHub, and keep the service available longer.

## Pre-Computed Bottom Views

See [cache.go](https://github.com/adamjeanlaurent/github-api-read-cache-service/blob/main/cache/cache.go#L160).

The sorting of the repos by issues / forks / update time / stars is done only when the cache is being warmed. There's no need to sort these views on every request we get. When a request comes in for the bottom N of a view, we can just return the last N values in the corresponding sorted array.

This makes the requesting of bottom N views very quick, and it's just a memory read with no additional processing, 



