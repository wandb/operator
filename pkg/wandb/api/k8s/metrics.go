package k8s

import (
	"net/http"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

func getNodesWithMetrics(clientset *kubernetes.Clientset, metricsClientset *metricsclientset.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeList, err := clientset.CoreV1().Nodes().List(c, metav1.ListOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		nodeMetricsList, err := metricsClientset.MetricsV1beta1().NodeMetricses().List(c, metav1.ListOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		nodeMetricsMap := make(map[string]*metrics.NodeMetrics)
		for _, nodeMetrics := range nodeMetricsList.Items {
			nodeMetricsMap[nodeMetrics.Name] = &nodeMetrics
		}

		nodes := make([]map[string]interface{}, 0, len(nodeList.Items))
		for _, node := range nodeList.Items {
			nodeMetrics := nodeMetricsMap[node.Name]

			if nodeMetrics == nil {
				continue
			}

			cpuUsage := nodeMetrics.Usage[corev1.ResourceCPU]
			memoryUsage := nodeMetrics.Usage[corev1.ResourceMemory]

			cpuTotal, _ := (node.Status.Capacity.Cpu()).AsInt64()
			cpuU, _ := (&cpuUsage).AsInt64()
			memoryTotal, _ := node.Status.Capacity.Memory().AsInt64()
			memoryU, _ := (&memoryUsage).AsInt64()

			nodes = append(nodes, map[string]interface{}{
				"name":   node.Name,
				"status": node.Status,
				"cpu": map[string]interface{}{
					"total": cpuTotal,
					"used":  cpuU,
				},
				"memory": map[string]interface{}{
					"total": memoryTotal,
					"used":  memoryU,
				},
			})
		}

		c.JSON(http.StatusOK, gin.H{"items": nodes})
	}
}
