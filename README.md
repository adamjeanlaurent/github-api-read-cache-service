# API Read Cache

## Running Locally

### Clone the Repository
```git clone https://github.com/adamjeanlaurent/github-api-read-cache-service.git```

```cd api-read-cache-service```

### Building
Run the build script to build the service is all flavors.
```./scripts/build.sh```

### Running
After the build is finished the follow binaries are built. ```(server-linux-amd, server-linux-arm, server-mac-amd, server-mac-arm, server-windows-amd.exe, server-windows-arm.exe)```

Run the appropriate binary, port is a required option. Optionally set the environment variable GITHUB_API_TOKEN with a github API token.

ex. ```./bin/server/server-mac-arm --port=7101```

ex. ```GITHUB_API_TOKEN=xyz123 ./bin/server/server-mac-arm --port=7101```

# Design Decisions

## Dedicated Thread for Cache Warming 
See [cache.StartSyncLoop()](https://github.com/adamjeanlaurent/github-api-read-cache-service/blob/main/cache/cache.go#L58).

The server has a long-living dedicated thread that every 10 minutes (and at server startup), warms the cache via fetching /orgs/Netflix, /orgs/Netflix/members/, /orgs/Netflix/repos from the GitHub API, flattening the reponeses, and computing bottom repo views by forks, issues, update time, and stars.

The fetched and computed data is cached in memory, to be served when users ask for it.

I chose cache warming for 2 reasons. 

1. Lowers client latency to our service, as the cached data will always be available and accurate by at least the last 10 minutes. 10 Minutes is sort of an arbitrary TTL, it could be longer if we're okay with staler data.
2. Limits the amount of calls we make to the actual github API to a only a couple every 10 minutes, meaning we won't cause burden to github.

## Pre-Computed Bottom Views

## Backoff 

## Intial Cache Fetch Retries
