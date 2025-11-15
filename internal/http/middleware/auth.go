package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"violation-service/internal/auth"
	"violation-service/internal/model"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer"
	principalContextKey = "principal"
)

func Auth(parser *auth.Parser) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader(authorizationHeader)
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header missing"})
			return
		}
		parts := strings.SplitN(raw, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], bearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}
		claims, err := parser.Parse(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		principal := model.Principal{
			UserID:   claims.UserID,
			OrgID:    claims.OrgID,
			Role:     claims.Role,
			DriverID: claims.DriverID,
		}
		c.Set(principalContextKey, principal)
		c.Next()
	}
}

func MustPrincipal(c *gin.Context) (model.Principal, bool) {
	value, exists := c.Get(principalContextKey)
	if !exists {
		return model.Principal{}, false
	}
	principal, ok := value.(model.Principal)
	if !ok {
		return model.Principal{}, false
	}
	return principal, true
}
