package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"official-account-service/internal/agentmcp"
)

const (
	defaultMCPTransport = "stdio"
	httpMCPTransport    = "streamable-http"
	defaultMCPAddr      = "127.0.0.1:8091"
	defaultMCPPath      = "/mcp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Printf("official account mcp server stopped: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	client, err := agentmcp.NewClient(agentmcp.ConfigFromEnv())
	if err != nil {
		return fmt.Errorf("init official account mcp client: %w", err)
	}
	server := agentmcp.NewServer(client, agentmcp.ServerConfig{
		AllowedRoot: os.Getenv("OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT"),
	})

	transport := strings.ToLower(strings.TrimSpace(os.Getenv("OFFICIAL_ACCOUNT_MCP_TRANSPORT")))
	addr := strings.TrimSpace(os.Getenv("OFFICIAL_ACCOUNT_MCP_ADDR"))
	if transport == "" && addr != "" {
		transport = httpMCPTransport
	}
	if transport == "" {
		transport = defaultMCPTransport
	}

	switch transport {
	case defaultMCPTransport:
		return server.Run(ctx, &mcp.StdioTransport{})
	case httpMCPTransport, "http":
		if addr == "" {
			addr = defaultMCPAddr
		}
		token := strings.TrimSpace(os.Getenv("OFFICIAL_ACCOUNT_MCP_TOKEN"))
		if token == "" {
			return fmt.Errorf("OFFICIAL_ACCOUNT_MCP_TOKEN must be set for %s transport", httpMCPTransport)
		}
		return runStreamableHTTP(ctx, server, streamableHTTPConfig{
			Addr:  addr,
			Path:  envOrDefault("OFFICIAL_ACCOUNT_MCP_PATH", defaultMCPPath),
			Token: token,
		})
	default:
		return fmt.Errorf("unsupported OFFICIAL_ACCOUNT_MCP_TRANSPORT %q", transport)
	}
}

type streamableHTTPConfig struct {
	Addr  string
	Path  string
	Token string
}

func runStreamableHTTP(ctx context.Context, server *mcp.Server, cfg streamableHTTPConfig) error {
	cfg.Path = normalizeMCPPath(cfg.Path)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           newStreamableHTTPMux(server, cfg),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("official account mcp streamable-http listening on %s%s", cfg.Addr, cfg.Path)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newStreamableHTTPMux(server *mcp.Server, cfg streamableHTTPConfig) http.Handler {
	path := normalizeMCPPath(cfg.Path)
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		JSONResponse:               true,
		DisableLocalhostProtection: true,
	})
	protected := auth.RequireBearerToken(staticTokenVerifier(cfg.Token), nil)(streamable)

	mux := http.NewServeMux()
	mux.Handle(path, protected)
	mux.HandleFunc(path+"/healthz", healthz)
	mux.HandleFunc("/healthz", healthz)
	return mux
}

func staticTokenVerifier(expected string) auth.TokenVerifier {
	expectedHash := sha256.Sum256([]byte(expected))
	return func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		actualHash := sha256.Sum256([]byte(token))
		if subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) != 1 {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{
			Scopes:     []string{"official-account:mcp"},
			Expiration: time.Now().Add(24 * time.Hour),
			UserID:     "official-account-agent",
		}, nil
	}
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func normalizeMCPPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultMCPPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	return path
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
