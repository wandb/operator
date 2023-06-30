package auth

import (
	"github.com/gin-gonic/gin"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AuthRequired(client client.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		pwd, err := getPasswordFromRequest(c.Request)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}

		if !isPassword(c, client, pwd) {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}
