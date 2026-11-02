package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterUserAIRoutes registers TransitAI's generation endpoints separately
// from the management routes reused by the private MCP router.
func RegisterUserAIRoutes(
	v1 *gin.RouterGroup,
	userAI *handler.UserAIHandler,
	jwtAuth middleware.JWTAuthMiddleware,
	apiKeyAuth middleware.APIKeyAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	opsService *service.OpsService,
	settingService *service.SettingService,
	cfg *config.Config,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	user := v1.Group("/user")
	user.Use(gin.HandlerFunc(jwtAuth))
	user.Use(middleware.BackendModeUserGuard(settingService))
	user.Use(panelRateLimiter.Global())
	user.Use(gin.HandlerFunc(auditLog))
	user.GET("/ai/models", userAI.Models)
	user.GET("/images/models", userAI.ImageModels)
	user.GET("/image/history", userAI.ListImageHistory)
	user.POST("/files/upload", middleware.RequestBodyLimit(handler.UserAIUploadRequestBodyLimit), userAI.UploadImage)
	user.GET("/chat/conversations", userAI.ListChatConversations)
	user.POST("/chat/conversations", userAI.CreateChatConversation)
	user.PATCH("/chat/conversations/:id/title", userAI.UpdateChatConversationTitle)
	user.DELETE("/chat/conversations/:id", userAI.DeleteChatConversation)

	bodyLimit := middleware.RequestBodyLimit(cfg.Gateway.MaxBodySize)
	clientRequestID := middleware.ClientRequestID()
	opsErrorLogger := handler.OpsErrorLoggerMiddleware(opsService)
	endpointNorm := handler.InboundEndpointMiddleware()
	requireGroupAnthropic := middleware.RequireGroupAssignment(settingService, middleware.AnthropicErrorWriter)
	user.POST("/chat/completions",
		bodyLimit,
		clientRequestID,
		opsErrorLogger,
		endpointNorm,
		userAI.PrepareChatCompletionsProxy,
		gin.HandlerFunc(apiKeyAuth),
		requireGroupAnthropic,
		userAI.ChatCompletions,
	)
	user.POST("/images/generations",
		bodyLimit,
		clientRequestID,
		opsErrorLogger,
		endpointNorm,
		userAI.PrepareImageGenerationsProxy,
		gin.HandlerFunc(apiKeyAuth),
		requireGroupAnthropic,
		userAI.ImageGenerations,
	)
	user.POST("/images/edits",
		bodyLimit,
		clientRequestID,
		opsErrorLogger,
		endpointNorm,
		userAI.PrepareImageEditsProxy,
		gin.HandlerFunc(apiKeyAuth),
		requireGroupAnthropic,
		userAI.ImageGenerations,
	)
}
