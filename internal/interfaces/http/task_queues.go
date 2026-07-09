package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/taskqueue"
)

type taskQueueURI struct {
	Queue string `uri:"queue" binding:"required"`
}

type archivedTaskURI struct {
	Queue  string `uri:"queue" binding:"required"`
	TaskID string `uri:"task_id" binding:"required"`
}

type archivedTaskQuery struct {
	Limit int `form:"limit" binding:"omitempty,gte=0,lte=100"`
}

type archivedTaskResponse struct {
	ID           string            `json:"id"`
	Queue        string            `json:"queue"`
	Type         string            `json:"type"`
	Payload      map[string]string `json:"payload"`
	Retried      int               `json:"retried"`
	MaxRetry     int               `json:"max_retry"`
	LastError    string            `json:"last_error"`
	LastFailedAt time.Time         `json:"last_failed_at"`
}

func registerTaskQueueRoutes(r gin.IRouter, service *application.TaskQueueService) {
	r.GET("/task-queues/:queue/archived-tasks", func(c *gin.Context) {
		tenant, ok := bindTenant(c)
		if !ok {
			return
		}
		var uri taskQueueURI
		if err := c.ShouldBindUri(&uri); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		var query archivedTaskQuery
		if err := c.ShouldBindQuery(&query); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		tasks, err := service.ListArchivedTasks(c.Request.Context(), application.ListArchivedTasksInput{
			TenantID: tenant, Queue: uri.Queue, Limit: query.Limit,
		})
		if !writeServiceError(c, err) {
			return
		}
		out := make([]archivedTaskResponse, 0, len(tasks))
		for _, task := range tasks {
			out = append(out, toArchivedTaskResponse(task))
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})
	r.POST("/task-queues/:queue/archived-tasks/:task_id/retry", func(c *gin.Context) {
		tenant, ok := bindTenant(c)
		if !ok {
			return
		}
		var uri archivedTaskURI
		if err := c.ShouldBindUri(&uri); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		err := service.RetryArchivedTask(c.Request.Context(), application.RetryArchivedTaskInput{
			TenantID: tenant, Queue: uri.Queue, TaskID: uri.TaskID,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "queued"})
	})
}

func toArchivedTaskResponse(task taskqueue.ArchivedTask) archivedTaskResponse {
	return archivedTaskResponse{
		ID: task.ID, Queue: task.Queue, Type: task.Type, Payload: task.Payload,
		Retried: task.Retried, MaxRetry: task.MaxRetry, LastError: redactArchivedTaskLastError(task.LastError), LastFailedAt: task.LastFailedAt,
	}
}

func redactArchivedTaskLastError(message string) string {
	lower := strings.ToLower(message)
	for _, marker := range []string{"token", "secret", "refresh", "raw_payload", "raw callback"} {
		if strings.Contains(lower, marker) {
			return "[REDACTED]"
		}
	}
	return message
}
