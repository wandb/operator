package k8s

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"
)

func getConfig() (*rest.Config, *kubernetes.Clientset, *metricsclient.Clientset, error) {
	config := ctrl.GetConfigOrDie()

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	metricsClientSet, err := metricsclient.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	return config, clientset, metricsClientSet, nil
}

func Routes(router *gin.RouterGroup) {
	_, clientset, metrics, err := getConfig()
	if err != nil {
		panic(err)
	}

	router.GET("/nodes", getNodesWithMetrics(clientset, metrics))
	router.GET("/:namespace/pods", func(c *gin.Context) {
		namespace := c.Param("namespace")
		pods, err := clientset.CoreV1().Pods(namespace).List(c, metav1.ListOptions{})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, pods)
	})

	router.GET("/:namespace/pods/:name/logs", func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		request := clientset.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{})
		podLogs, err := request.Stream(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		defer podLogs.Close()

		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, podLogs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.String(http.StatusOK, buf.String())
	})

	router.GET("/:namespace/deployments", func(c *gin.Context) {
		namespace := c.Param("namespace")
		deployments, err := clientset.AppsV1().Deployments(namespace).List(c, metav1.ListOptions{})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, deployments)
	})

	router.GET("/:namespace/stateful-sets", func(c *gin.Context) {
		namespace := c.Param("namespace")
		ss, err := clientset.AppsV1().StatefulSets(namespace).List(c, metav1.ListOptions{})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, ss)
	})
}
