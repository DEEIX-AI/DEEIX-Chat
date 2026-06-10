package main

import (
	"log"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/docs"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/cli"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo"
)

// @title DEEIX Chat API
// @version 0.2.3
// @description DEEIX Chat 后端 API 文档
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	docs.SwaggerInfo.Version = buildinfo.ResolveVersion()
	if err := cli.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
