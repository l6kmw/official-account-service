package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/config"
	domainarticle "official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	domainmaterial "official-account-service/internal/domain/material"
	domainpublish "official-account-service/internal/domain/publish"
	"official-account-service/internal/domain/wechatcallback"
	infracrypto "official-account-service/internal/infra/crypto"
	"official-account-service/internal/infra/logging"
	"official-account-service/internal/infra/persistence/memory"
	"official-account-service/internal/infra/persistence/postgres"
	infraqueue "official-account-service/internal/infra/queue"
	infraredis "official-account-service/internal/infra/redis"
	"official-account-service/internal/infra/wechat"
	httpapi "official-account-service/internal/interfaces/http"
)

func main() {
	configPath := flag.String("config", config.DefaultPath, "YAML config file path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	logger, err := logging.New(cfg.LogLevel)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	authorizationService, accountService, articleService, materialService, publishService, tokenService, callbackService, closeStore, err := buildServices(ctx, cfg)
	if err != nil {
		logger.Fatal("init persistence", logging.Error(err))
	}
	defer closeStore()
	closeTaskWorkers, err := startTaskWorkers(cfg, publishService, tokenService, logger)
	if err != nil {
		logger.Fatal("start task workers", logging.Error(err))
	}
	defer closeTaskWorkers()
	taskQueueService, closeTaskQueueService, err := buildTaskQueueService(cfg)
	if err != nil {
		logger.Fatal("init task queue service", logging.Error(err))
	}
	defer closeTaskQueueService()

	deps := buildHTTPDependencies(
		logger,
		authorizationService,
		accountService,
		articleService,
		materialService,
		publishService,
		tokenService,
		callbackService,
		taskQueueService,
	)
	deps.AdminAPIKey = cfg.AdminAPIKey
	deps.AdminUsername = cfg.AdminUsername
	deps.AdminPasswordHash = cfg.AdminPasswordHash
	deps.AdminSessionSecret = cfg.AdminSessionSecret
	deps.MCPToken = cfg.MCPToken
	deps.MCPPath = cfg.MCPPath
	officialContentService, err := buildOfficialContentService(cfg, tokenService)
	if err != nil {
		logger.Fatal("init official content service", logging.Error(err))
	}
	deps.OfficialContent = officialContentService
	permanentMaterialService, err := buildPermanentMaterialService(cfg, tokenService)
	if err != nil {
		logger.Fatal("init permanent material service", logging.Error(err))
	}
	deps.PermanentMaterials = permanentMaterialService
	router := httpapi.NewRouter(deps)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown http server", logging.Error(err))
		}
	}()

	logger.Info("http server starting", logging.String("addr", cfg.HTTPAddr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("serve http", logging.Error(err))
	}
}

func buildServices(ctx context.Context, cfg config.Config) (*application.AuthorizationService, *application.AccountService, *application.ArticleService, *application.MaterialService, *application.PublishService, *application.TokenService, *application.CallbackService, func(), error) {
	if strings.TrimSpace(cfg.DBDSN) == "" {
		store := memory.NewStore(time.Now)
		componentClient, err := buildWeChatComponentClient(store, cfg)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		callbackDecryptor, err := buildComponentCallbackDecryptor(cfg)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		refreshTokens, err := buildRefreshTokenEncryptor(cfg)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		preAuthCodes, authorizers := componentClientDependencies(componentClient)
		tokenRefreshLocker, accessTokenCache, closeTokenRefreshInfra, err := buildTokenRefreshInfrastructure(ctx, cfg)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		tokenRefreshScheduler, closeTokenRefreshScheduler, err := buildTokenRefreshTaskQueue(cfg)
		if err != nil {
			closeTokenRefreshInfra()
			return nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		tokenService := application.NewTokenServiceWithLockerCacheAndScheduler(store, authorizers, refreshTokens, tokenRefreshLocker, accessTokenCache, tokenRefreshScheduler, time.Now)
		statusSync, closeStatusSync, err := buildPublishStatusSyncQueue(cfg)
		if err != nil {
			closeTokenRefreshScheduler()
			closeTokenRefreshInfra()
			return nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		materialService, err := buildMaterialService(cfg, store, tokenService)
		if err != nil {
			closeStatusSync()
			closeTokenRefreshScheduler()
			closeTokenRefreshInfra()
			return nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		publishService, err := buildPublishService(cfg, store, tokenService, statusSync)
		if err != nil {
			closeStatusSync()
			closeTokenRefreshScheduler()
			closeTokenRefreshInfra()
			return nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		callbackService := buildCallbackService(cfg, store, publishService, callbackDecryptor)
		return application.NewAuthorizationServiceWithSecureAuthorizationFlow(store, store, store, preAuthCodes, callbackDecryptor, authorizers, refreshTokens, time.Now), application.NewAccountService(store), application.NewArticleServiceWithAuthorizerRepository(store, store), materialService, publishService, tokenService, callbackService, func() {
			closeStatusSync()
			closeTokenRefreshScheduler()
			closeTokenRefreshInfra()
		}, nil
	}
	store, err := postgres.Open(ctx, cfg.DBDSN)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("open postgres store: %w", err)
	}
	componentClient, err := buildWeChatComponentClient(store, cfg)
	if err != nil {
		_ = store.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	callbackDecryptor, err := buildComponentCallbackDecryptor(cfg)
	if err != nil {
		_ = store.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	refreshTokens, err := buildRefreshTokenEncryptor(cfg)
	if err != nil {
		_ = store.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	preAuthCodes, authorizers := componentClientDependencies(componentClient)
	tokenRefreshLocker, accessTokenCache, closeTokenRefreshInfra, err := buildTokenRefreshInfrastructure(ctx, cfg)
	if err != nil {
		_ = store.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	tokenRefreshScheduler, closeTokenRefreshScheduler, err := buildTokenRefreshTaskQueue(cfg)
	if err != nil {
		closeTokenRefreshInfra()
		_ = store.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	tokenService := application.NewTokenServiceWithLockerCacheAndScheduler(store, authorizers, refreshTokens, tokenRefreshLocker, accessTokenCache, tokenRefreshScheduler, time.Now)
	statusSync, closeStatusSync, err := buildPublishStatusSyncQueue(cfg)
	if err != nil {
		closeTokenRefreshScheduler()
		closeTokenRefreshInfra()
		_ = store.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	materialService, err := buildMaterialService(cfg, store, tokenService)
	if err != nil {
		closeStatusSync()
		closeTokenRefreshScheduler()
		closeTokenRefreshInfra()
		_ = store.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	publishService, err := buildPublishService(cfg, store, tokenService, statusSync)
	if err != nil {
		closeStatusSync()
		closeTokenRefreshScheduler()
		closeTokenRefreshInfra()
		_ = store.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	callbackService := buildCallbackService(cfg, store, publishService, callbackDecryptor)
	return application.NewAuthorizationServiceWithSecureAuthorizationFlow(store, store, store, preAuthCodes, callbackDecryptor, authorizers, refreshTokens, time.Now), application.NewAccountService(store), application.NewArticleServiceWithAuthorizerRepository(store, store), materialService, publishService, tokenService, callbackService, func() {
		closeStatusSync()
		closeTokenRefreshScheduler()
		closeTokenRefreshInfra()
		_ = store.Close()
	}, nil
}

func componentClientDependencies(client *wechat.ComponentClient) (authorization.PreAuthCodeCreator, authorization.AuthorizerClient) {
	if client == nil {
		return nil, nil
	}
	return client, client
}

func buildWeChatComponentClient(tickets authorization.ComponentVerifyTicketRepository, cfg config.Config) (*wechat.ComponentClient, error) {
	if strings.TrimSpace(cfg.WeChatComponentAppSecret) == "" {
		return nil, nil
	}
	client, err := wechat.NewComponentClient(tickets, wechat.ComponentClientConfig{
		ComponentAppSecret: cfg.WeChatComponentAppSecret,
		BaseURL:            cfg.WeChatAPIBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("init wechat component client: %w", err)
	}
	return client, nil
}

func buildComponentCallbackDecryptor(cfg config.Config) (authorization.ComponentCallbackDecryptor, error) {
	hasToken := strings.TrimSpace(cfg.WeChatComponentToken) != ""
	hasAESKey := strings.TrimSpace(cfg.WeChatComponentAESKey) != ""
	if !hasToken && !hasAESKey {
		return nil, nil
	}
	if !hasToken || !hasAESKey {
		return nil, fmt.Errorf("init wechat component callback crypto: %w", authorization.ErrComponentCallbackCryptoUnavailable)
	}
	crypto, err := wechat.NewComponentCallbackCrypto(wechat.ComponentCallbackCryptoConfig{
		Token:          cfg.WeChatComponentToken,
		EncodingAESKey: cfg.WeChatComponentAESKey,
		ComponentAppID: cfg.WeChatComponentAppID,
	})
	if err != nil {
		return nil, fmt.Errorf("init wechat component callback crypto: %w", err)
	}
	return crypto, nil
}

func buildRefreshTokenEncryptor(cfg config.Config) (authorization.RefreshTokenCodec, error) {
	if strings.TrimSpace(cfg.WeChatRefreshTokenKey) == "" {
		return nil, nil
	}
	encryptor, err := infracrypto.NewRefreshTokenEncryptor(cfg.WeChatRefreshTokenKey)
	if err != nil {
		return nil, fmt.Errorf("init refresh token encryptor: %w", err)
	}
	return encryptor, nil
}

func buildTokenRefreshInfrastructure(ctx context.Context, cfg config.Config) (authorization.TokenRefreshLocker, authorization.AuthorizerAccessTokenCache, func(), error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return nil, nil, func() {}, nil
	}
	client, err := infraredis.OpenClient(ctx, cfg.RedisAddr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("init redis token refresh infrastructure: %w", err)
	}
	return infraredis.NewTokenRefreshLocker(client), infraredis.NewAuthorizerAccessTokenCache(client, time.Now), func() { _ = client.Close() }, nil
}

func buildPublishStatusSyncQueue(cfg config.Config) (domainpublish.StatusSyncScheduler, func(), error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return nil, func() {}, nil
	}
	scheduler, err := infraqueue.NewPublishStatusSyncQueue(infraqueue.PublishStatusSyncQueueConfig{RedisAddr: cfg.RedisAddr})
	if err != nil {
		return nil, nil, fmt.Errorf("init publish status sync queue: %w", err)
	}
	return scheduler, func() { _ = scheduler.Close() }, nil
}

func buildTokenRefreshTaskQueue(cfg config.Config) (authorization.TokenRefreshScheduler, func(), error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return nil, func() {}, nil
	}
	scheduler, err := infraqueue.NewTokenRefreshQueue(infraqueue.TokenRefreshQueueConfig{RedisAddr: cfg.RedisAddr})
	if err != nil {
		return nil, nil, fmt.Errorf("init token refresh task queue: %w", err)
	}
	return scheduler, func() { _ = scheduler.Close() }, nil
}

func buildTaskQueueService(cfg config.Config) (*application.TaskQueueService, func(), error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return nil, func() {}, nil
	}
	inspector, err := infraqueue.NewTaskInspector(cfg.RedisAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("init task queue inspector: %w", err)
	}
	return application.NewTaskQueueService(inspector), func() { _ = inspector.Close() }, nil
}

func buildMaterialService(cfg config.Config, store interface {
	domainmaterial.Repository
	domainarticle.Repository
}, tokens *application.TokenService) (*application.MaterialService, error) {
	if strings.TrimSpace(cfg.WeChatComponentAppID) == "" {
		return application.NewMaterialService(store, store, wechat.DisabledMaterialUploader{}), nil
	}
	uploader, err := wechat.NewMaterialUploader(wechat.MaterialUploaderConfig{BaseURL: cfg.WeChatAPIBaseURL})
	if err != nil {
		return nil, fmt.Errorf("init wechat material uploader: %w", err)
	}
	return application.NewMaterialServiceWithTokenProvider(store, store, uploader, tokens, cfg.WeChatComponentAppID), nil
}

func buildPublishService(cfg config.Config, store interface {
	domainpublish.Repository
	domainarticle.Repository
	domainmaterial.Repository
}, tokens *application.TokenService, statusSync domainpublish.StatusSyncScheduler) (*application.PublishService, error) {
	if strings.TrimSpace(cfg.WeChatComponentAppID) == "" {
		return application.NewPublishService(store, store, time.Now), nil
	}
	publisher, err := wechat.NewPublisher(wechat.PublisherConfig{BaseURL: cfg.WeChatAPIBaseURL})
	if err != nil {
		return nil, fmt.Errorf("init wechat publisher: %w", err)
	}
	return application.NewPublishServiceWithPublisherAndStatusSync(store, store, store, publisher, tokens, cfg.WeChatComponentAppID, statusSync, time.Now), nil
}

func buildOfficialContentService(cfg config.Config, tokens *application.TokenService) (*application.OfficialContentService, error) {
	if strings.TrimSpace(cfg.WeChatComponentAppID) == "" {
		return application.NewOfficialContentService(nil, tokens, ""), nil
	}
	reader, err := wechat.NewPublisher(wechat.PublisherConfig{BaseURL: cfg.WeChatAPIBaseURL})
	if err != nil {
		return nil, fmt.Errorf("init wechat official content reader: %w", err)
	}
	return application.NewOfficialContentService(reader, tokens, cfg.WeChatComponentAppID), nil
}

func buildPermanentMaterialService(cfg config.Config, tokens *application.TokenService) (*application.PermanentMaterialService, error) {
	if strings.TrimSpace(cfg.WeChatComponentAppID) == "" {
		return application.NewPermanentMaterialService(nil, tokens, ""), nil
	}
	manager, err := wechat.NewMaterialUploader(wechat.MaterialUploaderConfig{BaseURL: cfg.WeChatAPIBaseURL})
	if err != nil {
		return nil, fmt.Errorf("init wechat permanent material manager: %w", err)
	}
	return application.NewPermanentMaterialService(manager, tokens, cfg.WeChatComponentAppID), nil
}

func buildCallbackService(cfg config.Config, store interface {
	wechatcallback.Repository
	authorization.AuthorizerTenantBindingRepository
}, publishes *application.PublishService, decryptor authorization.ComponentCallbackDecryptor) *application.CallbackService {
	return application.NewCallbackService(store, store, publishes, decryptor, cfg.WeChatComponentAppID, time.Now)
}

func startTaskWorkers(cfg config.Config, publishes *application.PublishService, tokens *application.TokenService, logger *zap.Logger) (func(), error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return func() {}, nil
	}
	publishTaskService := application.NewPublishStatusTaskService(publishes)
	tokenTaskService := application.NewTokenRefreshTaskService(tokens)
	mux := asynq.NewServeMux()
	if err := infraqueue.RegisterPublishStatusSyncHandler(mux, publishTaskService); err != nil {
		return nil, fmt.Errorf("register publish status sync handler: %w", err)
	}
	if err := infraqueue.RegisterTokenRefreshHandler(mux, tokenTaskService); err != nil {
		return nil, fmt.Errorf("register token refresh handler: %w", err)
	}
	server := asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.RedisAddr}, asynq.Config{
		Concurrency:     2,
		Queues:          map[string]int{infraqueue.PublishQueueName: 1, infraqueue.TokenQueueName: 1},
		Logger:          infraqueue.NewZapLogger(logger),
		ShutdownTimeout: 5 * time.Second,
	})
	if err := server.Start(mux); err != nil {
		return nil, fmt.Errorf("start asynq server: %w", err)
	}
	return func() {
		server.Stop()
		server.Shutdown()
	}, nil
}
