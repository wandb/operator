package auth

import (
	"github.com/gin-gonic/gin"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AuthRequired(client client.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(CookieAuthName)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}

		if !isPassword(c, client, cookie.Value) {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}
