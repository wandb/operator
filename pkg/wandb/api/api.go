package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func enableCORS(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
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

// func isPasswordValid(settings *settings.Settings, w http.ResponseWriter, r *http.Request) bool {
// 	password := r.Header.Get("Authorization")
// 	if password == "" {
// 		http.Error(w, "Authorization password must be provided", http.StatusUnauthorized)
// 		return false
// 	}

// 	hashedPassword := strings.Replace(password, "Password ", "", 1)
// 	if !settings.IsPassword(hashedPassword) {
// 		http.Error(w, "Invalid password", http.StatusForbidden)
// 		return false
// 	}
// 	return true
// }

// func passwordAuthMiddleware(settings *settings.Settings, handler http.HandlerFunc) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		if !isPasswordValid(settings, w, r) {
// 			return
// 		}

// 		// If the password is valid, call the handler
// 		handler.ServeHTTP(w, r)
// 	}
// }

func New(log logr.Logger, c client.Client, scheme *runtime.Scheme) {
	ctx := context.Background()

	// settings := settings.New(c)

	log.Info("Initalizing API server")

	api := http.NewServeMux()

	// api.HandleFunc("/api/v1/password", func(w http.ResponseWriter, r *http.Request) {
	// 	if r.Method == "POST" {
	// 		if !isPasswordValid(settings, w, r) {
	// 			return
	// 		}

	// 		decoder := json.NewDecoder(r.Body)
	// 		var pw string
	// 		err := decoder.Decode(&pw)
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 		defer r.Body.Close()
	// 		settings.SetPassword(pw)
	// 		_, _ = w.Write([]byte("ok"))
	// 		return
	// 	}

	// 	if r.Method == "GET" {
	// 		val := json.Marshal(map[string]interface{}{

	// 		})
	// 		_, _ = w.Write([]byte(val))
	// 		return
	// 	}
	// })

	api.HandleFunc("/api/v1/config/latest", func(w http.ResponseWriter, r *http.Request) {
		configName := "wandb-config-latest"
		configNamespace := "default"
		latest, _ := config.GetFromConfigMap(ctx, c, configName, configNamespace)

		if r.Method == "POST" {
			decoder := json.NewDecoder(r.Body)
			var cfg interface{}
			err := decoder.Decode(&cfg)
			if err != nil {
				panic(err)
			}
			defer r.Body.Close()
			config.UpdateWithConfigMap(
				ctx,
				c,
				scheme,

				configName,
				configNamespace,

				latest.Release,
				cfg,
			)
			_, _ = w.Write([]byte("ok"))
			return
		}

		if r.Method == "GET" {
			latest, _ := config.GetFromConfigMap(ctx, c, configName, configNamespace)
			js, _ := json.Marshal(latest.Config)
			_, _ = w.Write([]byte(js))
			return
		}
	})

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
		log.Info("Starting API server", "port", port)
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), corsAPI)
		if err != nil {
			log.Error(err, "Failed to start API server")
			os.Exit(1)
		}
	}()
}
