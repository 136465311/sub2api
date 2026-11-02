package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserAIRoutesRequireLoginBeforeGatewayAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiKeyAuthCalled := false
	RegisterUserAIRoutes(router.Group("/api/v1"), nil,
		middleware.JWTAuthMiddleware(func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }),
		middleware.APIKeyAuthMiddleware(func(c *gin.Context) { apiKeyAuthCalled = true; c.Next() }),
		middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		nil, nil, &config.Config{}, nil,
	)
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, "/ai/models"},
		{http.MethodGet, "/images/models"},
		{http.MethodGet, "/image/history"},
		{http.MethodPost, "/files/upload"},
		{http.MethodGet, "/chat/conversations"},
		{http.MethodPost, "/chat/conversations"},
		{http.MethodPatch, "/chat/conversations/1/title"},
		{http.MethodDelete, "/chat/conversations/1"},
		{http.MethodPost, "/chat/completions"},
		{http.MethodPost, "/images/generations"},
		{http.MethodPost, "/images/edits"},
	} {
		t.Run(endpoint.method+endpoint.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(endpoint.method, "/api/v1/user"+endpoint.path, nil))
			require.Equal(t, http.StatusUnauthorized, response.Code)
			require.False(t, apiKeyAuthCalled)
		})
	}
}
