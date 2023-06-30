package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/wandb/operator/pkg/wandb/api/auth"
	"github.com/wandb/operator/pkg/wandb/api/config"
	"github.com/wandb/operator/pkg/wandb/api/k8s"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(c client.Client, scheme *runtime.Scheme) {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"PUT", "PATCH", "GET", "POST", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length"},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			return true
		},
	}))

	apiV1 := r.Group("/api/v1")

	auth.Routes(apiV1.Group("/auth"), c)

	cfgRoutes := apiV1.Group("/config")
	cfgRoutes.Use(auth.AuthRequired(c))
	config.Routes(cfgRoutes, c, scheme)

	k8sRoutes := apiV1.Group("/k8s")
	k8sRoutes.Use(auth.AuthRequired(c))
	k8s.Routes(k8sRoutes)

	r.Static("/static", "./console/dist")
	r.NoRoute(func(c *gin.Context) {
		c.File("./console/dist/index.html")
	})

	r.Run(":9090")
}
