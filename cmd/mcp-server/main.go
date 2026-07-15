package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"flag"
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
	"official-account-service/internal/config"
)

const (
	defaultMCPTransport = "stdio"
	httpMCPTransport    = "streamable-http"
	defaultMCPAddr      = "127.0.0.1:8091"
	defaultMCPPath      = "/mcp"
)

func main() {
	configPath := flag.String("config", config.DefaultPath, "YAML config file path used as fallback for MCP settings")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *configPath); err != nil {
		log.Printf("official account mcp server stopped: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, configPath string) error {
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
		streamableConfig, err := mcpStreamableHTTPConfig(configPath)
		if err != nil {
			return err
		}
		if streamableConfig.Token == "" {
			return fmt.Errorf("OFFICIAL_ACCOUNT_MCP_TOKEN or mcp.token must be set for %s transport", httpMCPTransport)
		}
		streamableConfig.Addr = addr
		streamableConfig.Verifier = userAwareTokenVerifier(client, streamableConfig.Token)
		return runStreamableHTTP(ctx, server, streamableConfig)
	default:
		return fmt.Errorf("unsupported OFFICIAL_ACCOUNT_MCP_TRANSPORT %q", transport)
	}
}

func mcpStreamableHTTPConfig(configPath string) (streamableHTTPConfig, error) {
	token := strings.TrimSpace(os.Getenv("OFFICIAL_ACCOUNT_MCP_TOKEN"))
	path := strings.TrimSpace(os.Getenv("OFFICIAL_ACCOUNT_MCP_PATH"))
	if token != "" && path != "" {
		return streamableHTTPConfig{Path: path, Token: token}, nil
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		if token != "" {
			if path == "" {
				path = defaultMCPPath
			}
			return streamableHTTPConfig{Path: path, Token: token}, nil
		}
		return streamableHTTPConfig{}, fmt.Errorf("load MCP settings from config: %w", err)
	}
	if token := strings.TrimSpace(os.Getenv("OFFICIAL_ACCOUNT_MCP_TOKEN")); token != "" {
		return streamableHTTPConfig{Path: envOrDefault("OFFICIAL_ACCOUNT_MCP_PATH", cfg.MCPPath), Token: token}, nil
	}
	return streamableHTTPConfig{Path: envOrDefault("OFFICIAL_ACCOUNT_MCP_PATH", cfg.MCPPath), Token: strings.TrimSpace(cfg.MCPToken)}, nil
}

type streamableHTTPConfig struct {
	Addr     string
	Path     string
	Token    string
	Verifier auth.TokenVerifier
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
	verifier := cfg.Verifier
	if verifier == nil {
		verifier = staticTokenVerifier(cfg.Token)
	}
	protected := normalizeMCPAuthHeader(auth.RequireBearerToken(verifier, nil)(streamable))

	mux := http.NewServeMux()
	mux.Handle(path, protected)
	mux.HandleFunc(path+"/healthz", healthz)
	mux.HandleFunc("/healthz", healthz)
	return mux
}

func userAwareTokenVerifier(client *agentmcp.Client, staticToken string) auth.TokenVerifier {
	staticVerifier := staticTokenVerifier(staticToken)
	return func(ctx context.Context, token string, req *http.Request) (*auth.TokenInfo, error) {
		if info, err := staticVerifier(ctx, token, req); err == nil {
			return info, nil
		}
		if client == nil || !strings.HasPrefix(token, "oat_") {
			return nil, auth.ErrInvalidToken
		}
		user, err := client.ValidateAPIToken(ctx, token)
		if err != nil {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{
			Scopes:     []string{"official-account:mcp"},
			Expiration: time.Now().Add(5 * time.Minute),
			UserID:     user.UserID,
			Extra: map[string]any{
				agentmcp.CredentialKindExtraKey: agentmcp.UserAPICredentialKind,
				agentmcp.UserIDExtraKey:         user.UserID,
				agentmcp.ActorTypeExtraKey:      user.ActorType,
				agentmcp.AgentRecordIDExtraKey:  user.AgentRecordID,
				agentmcp.AgentIDExtraKey:        user.AgentID,
				agentmcp.AgentNameExtraKey:      user.AgentName,
				agentmcp.AgentPurposeExtraKey:   user.AgentPurpose,
			},
		}, nil
	}
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

func normalizeMCPAuthHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" {
			if token := firstHeaderValue(r, "X-API-Key", "X-MCP-Token"); token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
		} else if !strings.Contains(authHeader, " ") {
			r.Header.Set("Authorization", "Bearer "+authHeader)
		}
		next.ServeHTTP(w, r)
	})
}

func firstHeaderValue(r *http.Request, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
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
