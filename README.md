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
