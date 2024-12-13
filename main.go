package main

import (
	"fmt"

	"github.com/adamjeanlaurent/github-api-read-cache-service/server"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()

	if err != nil {
		fmt.Printf("Failed to init logger: %s", err.Error())
	}

	err = server.StartServer(logger)

	if err != nil {
		logger.Error("Failed to start server", zap.Error(err))
	}
}
