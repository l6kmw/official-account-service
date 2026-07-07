package main

import (
	"go.uber.org/zap"

	"official-account-service/internal/application"
	httpapi "official-account-service/internal/interfaces/http"
)

func provideHTTPDependencies(
	logger *zap.Logger,
	authorization *application.AuthorizationService,
	accounts *application.AccountService,
	articles *application.ArticleService,
	materials *application.MaterialService,
	publishes *application.PublishService,
	tokens *application.TokenService,
	callbacks *application.CallbackService,
	taskQueues *application.TaskQueueService,
	dashboard *application.DashboardService,
) httpapi.Dependencies {
	return httpapi.Dependencies{
		Logger:        logger,
		Authorization: authorization,
		Accounts:      accounts,
		Articles:      articles,
		Materials:     materials,
		Publishes:     publishes,
		Tokens:        tokens,
		Callbacks:     callbacks,
		TaskQueues:    taskQueues,
		Dashboard:     dashboard,
	}
}
