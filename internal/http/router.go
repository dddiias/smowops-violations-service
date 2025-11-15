package http

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter(handler *Handler, authMiddleware gin.HandlerFunc, env string) *gin.Engine {
	if env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"*"},
		ExposeHeaders:   []string{"Content-Type"},
		MaxAge:          12 * time.Hour,
	}))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	protected := router.Group("/")
	protected.Use(authMiddleware)
	{
		protected.GET("/violations", handler.listViolations)
		protected.GET("/violations/:id", handler.getViolation)
		protected.POST("/violations", handler.createViolation)
		protected.PUT("/violations/:id/status", handler.updateViolationStatus)

		protected.GET("/appeals", handler.listAppeals)
		protected.GET("/appeals/:id", handler.getAppeal)
		protected.POST("/violations/:id/appeals", handler.createAppeal)
		protected.POST("/appeals/:id/comments", handler.addAppealComment)
		protected.POST("/appeals/:id/actions", handler.actOnAppeal)
	}

	return router
}
