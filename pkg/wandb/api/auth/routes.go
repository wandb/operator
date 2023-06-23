package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PasswordInput struct {
	Password string `json:"password"`
}

func Routes(router *gin.RouterGroup, client client.Client) {
	router.GET("/profile", func(c *gin.Context) {
		pwd, _ := getPassword(c.Request)
		c.JSON(200, gin.H{
			"isPasswordSet": isPasswordSet(c, client),
			"isLoggedIn":    isPassword(c, client, pwd),
		})
	})

	router.POST("/password", func(c *gin.Context) {
		if isPasswordSet(c, client) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "password already set"})
			return
		}

		var input PasswordInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		setPassword(c, client, input.Password)
		c.JSON(200, gin.H{"message": "profile update"})
	})

	router.POST("/login", func(c *gin.Context) {
		var input PasswordInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		hr := 60 * 60
		dy := 24 * hr
		wk := 7 * dy
		c.SetCookie(CookieAuthName, input.Password, wk, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{"message": "logged in set"})
	})

	router.POST("/logout", func(c *gin.Context) {
		c.SetCookie(CookieAuthName, "", -1, "/", "", false, true)
		c.JSON(200, gin.H{"message": "profile"})
	})
}
