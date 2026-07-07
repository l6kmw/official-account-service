//go:build wireinject

package main

import (
	"github.com/google/wire"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	httpapi "official-account-service/internal/interfaces/http"
)

//go:generate go run github.com/google/wire/cmd/wire

func buildHTTPDependencies(
	logger *zap.Logger,
	authorization *application.AuthorizationService,
	accounts *application.AccountService,
	articles *application.ArticleService,
	materials *application.MaterialService,
	publishes *application.PublishService,
	tokens *application.TokenService,
	callbacks *application.CallbackService,
	taskQueues *application.TaskQueueService,
) httpapi.Dependencies {
	wire.Build(
		application.NewDashboardService,
		provideHTTPDependencies,
	)
	return httpapi.Dependencies{}
}
