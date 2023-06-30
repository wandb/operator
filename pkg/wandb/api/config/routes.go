package config

import (
	"fmt"

	"github.com/gin-gonic/gin"
	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/cdk8s"
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigInput struct {
	Config map[string]interface{} `json:"config"`
}

func Routes(router *gin.RouterGroup, cl client.Client, scheme *runtime.Scheme) {

	router.GET("/:namespace/:name/spec", func(c *gin.Context) {
		wandb := &apiv1.WeightsAndBiases{}
		objKey := client.ObjectKey{Name: c.Param("name"), Namespace: c.Param("namespace")}
		if err := cl.Get(c, objKey, wandb); err != nil {
			fmt.Println("wandb", err)
			c.JSON(404, gin.H{"error": "no wandb found"})
			return
		}
		c.JSON(200, map[string]interface{}{
			"wandb": wandb,
		})
	})

	router.GET("/:namespace/:name/applied", func(c *gin.Context) {
		name := c.Param("name") + "-config-latest"
		namespace := c.Param("namespace")
		latest, err := config.GetFromConfigMap(c, cl, name, namespace)
		fmt.Println("latest", err)
		if err != nil {
			c.JSON(404, gin.H{"error": "no config found"})
			return
		}

		wandb := &apiv1.WeightsAndBiases{}
		objKey := client.ObjectKey{Name: c.Param("name"), Namespace: namespace}
		if err := cl.Get(c, objKey, wandb); err != nil {
			fmt.Println("wandb", err)
			c.JSON(404, gin.H{"error": "no wandb found"})
			return
		}

		var license string
		if latest != nil {
			if l, exists := latest.Config["license"]; exists {
				if ls, ok := l.(string); ok {
					license = ls
				}
			}
		}
		if wandb.Spec.License != "" {
			license = wandb.Spec.License
		}

		applied := config.Merge(
			latest,
			cdk8s.Github(),
			cdk8s.Deployment(license),
			cdk8s.Operator(wandb, scheme),
		)

		c.JSON(200, map[string]interface{}{
			"directory": applied.Release.Directory(),
			"version":   applied.Release.Version(),
			"config":    applied.Config,
		})
	})

	router.GET("/:namespace/:name/latest", func(c *gin.Context) {
		name := c.Param("name") + "-config-latest"
		namespace := c.Param("namespace")
		latest, err := config.GetFromConfigMap(c, cl, name, namespace)
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
		latest, err := config.GetFromConfigMap(c, cl, name, namespace)
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
			c, cl, scheme,
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
