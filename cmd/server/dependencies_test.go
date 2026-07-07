package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
)

func TestBuildHTTPDependenciesWiresDashboard(t *testing.T) {
	logger := zap.NewNop()
	accounts := application.NewAccountService(nil)
	articles := application.NewArticleService(nil)
	publishes := application.NewPublishService(nil, nil, time.Now)

	deps := buildHTTPDependencies(
		logger,
		nil,
		accounts,
		articles,
		nil,
		publishes,
		nil,
		nil,
		nil,
	)

	require.Same(t, logger, deps.Logger)
	require.Same(t, accounts, deps.Accounts)
	require.Same(t, articles, deps.Articles)
	require.Same(t, publishes, deps.Publishes)
	require.NotNil(t, deps.Dashboard)
}
