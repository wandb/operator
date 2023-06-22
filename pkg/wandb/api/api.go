package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func enableCORS(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// Handle preflight requests, which are sent as HTTP OPTIONS requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
		} else {
			handler.ServeHTTP(w, r)
		}
	})
}

func clientListHandler(
	ctx context.Context,
	c client.Client,
	list client.ObjectList,
	opts ...client.ListOption,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.List(ctx, list, opts...)
		js, _ := json.Marshal(list)
		_, _ = w.Write([]byte(js))
	}
}

func New(log logr.Logger, c client.Client, scheme *runtime.Scheme) {
	ctx := context.Background()

	log.Info("Initializing API server")

	// TODO: make this configurable, this assumes it always deploy into default
	// namespace with name wandb
	namespace := "default"

	api := http.NewServeMux()
	pwd := &PasswordMiddleware{ctx, c, []string{
		"/api/v1/password",
		"/api/v1/viewer",
		"/api/v1/login",
	}}
	api.HandleFunc("/api/v1/password", pwd.Password())
	api.HandleFunc("/api/v1/login", pwd.Login())
	api.HandleFunc("/api/v1/viewer", pwd.Viewer())
	api.HandleFunc("/api/v1/logout", pwd.Logout())

	cfg := &ConfigMiddleware{ctx, c, scheme}
	api.HandleFunc("/api/v1/config/latest", cfg.Latest())

	api.HandleFunc("/api/v1/k8s/pods", clientListHandler(ctx, c, &corev1.PodList{}, client.InNamespace(namespace)))
	api.HandleFunc("/api/v1/k8s/services", clientListHandler(ctx, c, &corev1.ServiceList{}, client.InNamespace(namespace)))
	api.HandleFunc("/api/v1/k8s/stateful-sets", clientListHandler(ctx, c, &appsv1.StatefulSetList{}, client.InNamespace(namespace)))
	api.HandleFunc("/api/v1/k8s/deployments", clientListHandler(ctx, c, &appsv1.DeploymentList{}, client.InNamespace(namespace)))
	api.HandleFunc("/api/v1/k8s/nodes", clientListHandler(ctx, c, &corev1.NodeList{}))

	consoleUI := http.FileServer(http.Dir("console/dist"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.Error(w, "Page not found", http.StatusNotFound)
			return
		}
		consoleUI.ServeHTTP(w, r)
	})
	port := 9090

	go func() {
		corsAPI := enableCORS(api)
		authAPI := pwd.Auth(corsAPI)
		log.Info("Starting API server", "port", port)
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), authAPI)
		if err != nil {
			log.Error(err, "Failed to start API server")
			os.Exit(1)
		}
	}()
}
