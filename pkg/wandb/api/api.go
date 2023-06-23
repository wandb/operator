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

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// 	"os"
// 	"strings"

// 	"github.com/go-logr/logr"
// 	"github.com/wandb/operator/pkg/wandb/api/auth"
// 	appsv1 "k8s.io/api/apps/v1"
// 	corev1 "k8s.io/api/core/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	"k8s.io/client-go/kubernetes"
// 	"k8s.io/client-go/rest"
// 	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
// 	ctrl "sigs.k8s.io/controller-runtime"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// )

// func enableCORS(handler http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		origin := r.Header.Get("Origin")
// 		w.Header().Set("Access-Control-Allow-Origin", origin)
// 		w.Header().Set("Access-Control-Allow-Credentials", "true")
// 		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
// 		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

// 		// Handle preflight requests, which are sent as HTTP OPTIONS requests
// 		if r.Method == "OPTIONS" {
// 			w.WriteHeader(http.StatusOK)
// 		} else {
// 			handler.ServeHTTP(w, r)
// 		}
// 	})
// }

// func getConfig() (*rest.Config, *kubernetes.Clientset, *metricsclient.Clientset, error) {
// 	config := ctrl.GetConfigOrDie()

// 	clientset, err := kubernetes.NewForConfig(config)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	metricsClientSet, err := metricsclient.NewForConfig(config)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	return config, clientset, metricsClientSet, nil
// }

// func clientListHandler(
// 	ctx context.Context,
// 	c client.Client,
// 	list client.ObjectList,
// 	opts ...client.ListOption,
// ) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		c.List(ctx, list, opts...)
// 		js, _ := json.Marshal(list)
// 		_, _ = w.Write([]byte(js))
// 	}
// }

// func New(log logr.Logger, c client.Client, scheme *runtime.Scheme) {
// 	ctx := context.Background()

// 	log.Info("Initializing API server")

// 	// TODO: make this configurable, this assumes it always deploy into default
// 	// namespace with name wandb
// 	namespace := "default"

// 	_, _, _, err := getConfig()
// 	fmt.Println(err)

// 	api := http.NewServeMux()
// 	pwd := auth.New(
// 		ctx,
// 		c,
// 		[]string{
// 			"/api/v1/password",
// 			"/api/v1/viewer",
// 			"/api/v1/login",
// 		},
// 	)
// 	api.HandleFunc("/api/v1/password", pwd.Password())
// 	api.HandleFunc("/api/v1/login", pwd.Login())
// 	api.HandleFunc("/api/v1/viewer", pwd.Viewer())
// 	api.HandleFunc("/api/v1/logout", pwd.Logout())

// 	cfg := &ConfigMiddleware{ctx, c, scheme}
// 	api.HandleFunc("/api/v1/config/latest", cfg.Latest())

// 	api.HandleFunc("/api/v1/k8s/pods", clientListHandler(ctx, c, &corev1.PodList{}, client.InNamespace(namespace)))
// 	api.HandleFunc("/api/v1/k8s/services", clientListHandler(ctx, c, &corev1.ServiceList{}, client.InNamespace(namespace)))
// 	api.HandleFunc("/api/v1/k8s/stateful-sets", clientListHandler(ctx, c, &appsv1.StatefulSetList{}, client.InNamespace(namespace)))
// 	api.HandleFunc("/api/v1/k8s/deployments", clientListHandler(ctx, c, &appsv1.DeploymentList{}, client.InNamespace(namespace)))
// 	api.HandleFunc("/api/v1/k8s/nodes", clientListHandler(ctx, c, &corev1.NodeList{}))

// 	consoleUI := http.FileServer(http.Dir("console/dist"))
// 	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
// 		if strings.HasPrefix(r.URL.Path, "/api") {
// 			http.Error(w, "Page not found", http.StatusNotFound)
// 			return
// 		}
// 		consoleUI.ServeHTTP(w, r)
// 	})
// 	port := 9090

// 	go func() {
// 		corsAPI := enableCORS(api)
// 		authAPI := pwd.Auth(corsAPI)
// 		log.Info("Starting API server", "port", port)
// 		err := http.ListenAndServe(fmt.Sprintf(":%d", port), authAPI)
// 		if err != nil {
// 			log.Error(err, "Failed to start API server")
// 			os.Exit(1)
// 		}
// 	}()
// }

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
	// k8sRoutes.Use(auth.AuthRequired(c))
	k8s.Routes(k8sRoutes)

	r.Run(":9090")
}
