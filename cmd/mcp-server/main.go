package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"official-account-service/internal/agentmcp"
)

func main() {
	client, err := agentmcp.NewClient(agentmcp.ConfigFromEnv())
	if err != nil {
		fmt.Fprintf(os.Stderr, "init official account mcp client: %v\n", err)
		os.Exit(1)
	}
	server := agentmcp.NewServer(client, agentmcp.ServerConfig{
		AllowedRoot: os.Getenv("OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT"),
	})
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("official account mcp server stopped: %v", err)
		os.Exit(1)
	}
}
