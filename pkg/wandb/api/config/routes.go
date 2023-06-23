package config

import (
	"github.com/gin-gonic/gin"
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigInput struct {
	Config map[string]interface{} `json:"config"`
}

func Routes(router *gin.RouterGroup, client client.Client, scheme *runtime.Scheme) {
	router.GET("/:namespace/:name/latest", func(c *gin.Context) {
		name := c.Param("name") + "-config-latest"
		namespace := c.Param("namespace")
		latest, err := config.GetFromConfigMap(c, client, name, namespace)
		if err != nil {
			c.JSON(404, gin.H{"error": "no config found"})
			return
		}

		c.JSON(200, map[string]interface{}{
			"directory": latest.Release.Directory(),
			"version":   latest.Release.Version(),
			"config":    latest.Config,
		})
	})

	router.POST("/:namespace/:name/latest", func(c *gin.Context) {
		name := c.Param("name") + "-config-latest"
		namespace := c.Param("namespace")
		latest, err := config.GetFromConfigMap(c, client, name, namespace)
		if err != nil {
			c.JSON(404, gin.H{"error": "no config found"})
			return
		}

		var input ConfigInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		_, err = config.UpdateWithConfigMap(
			c, client, scheme,
			name, namespace,
			latest.Release, input.Config,
		)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"status": "ok"})
	})
}
