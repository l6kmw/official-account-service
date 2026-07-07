package api_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestOpenAPIYAMLIsParseableAndCoversRoutes(t *testing.T) {
	raw, err := os.ReadFile("openapi.yaml")
	require.NoError(t, err)

	var spec struct {
		OpenAPI string                 `yaml:"openapi"`
		Info    map[string]any         `yaml:"info"`
		Paths   map[string]interface{} `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	require.Equal(t, "3.0.3", spec.OpenAPI)
	require.NotEmpty(t, spec.Info["title"])

	for _, path := range []string{
		"/healthz",
		"/api/v1/healthz",
		"/wechat/component/callback",
		"/wechat/authorizer/{app_id}/callback",
		"/api/v1/wechat/authorization-url",
		"/api/v1/wechat/authorization-callback",
		"/api/v1/accounts",
		"/api/v1/accounts/{id}",
		"/api/v1/accounts/{id}/token-status",
		"/api/v1/articles",
		"/api/v1/articles/{id}",
		"/api/v1/materials/inline-images",
		"/api/v1/materials/covers",
		"/api/v1/articles/{id}/publish",
		"/api/v1/publish-records",
		"/api/v1/publish-records/{id}",
		"/api/v1/articles/{id}/publish-records",
		"/api/v1/publish-records/{id}/status",
		"/api/v1/publish-records/{id}/sync-status",
		"/api/v1/dashboard/stats",
		"/api/v1/task-queues/{queue}/archived-tasks",
		"/api/v1/task-queues/{queue}/archived-tasks/{task_id}/retry",
	} {
		require.Contains(t, spec.Paths, path)
	}
}
