package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMiddleware struct {
	ctx    context.Context
	c      client.Client
	scheme *runtime.Scheme
}

func (c ConfigMiddleware) Latest() http.HandlerFunc {
	namespace := "wandb"
	configName := "wandb-config-latest"
	return func(w http.ResponseWriter, r *http.Request) {
		latest, _ := config.GetFromConfigMap(c.ctx, c.c, configName, namespace)

		if r.Method == "POST" {
			decoder := json.NewDecoder(r.Body)
			var cfg map[string]interface{}
			err := decoder.Decode(&cfg)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(" \"error\": \"invalid format\" }"))
				return
			}
			defer r.Body.Close()
			config.UpdateWithConfigMap(
				c.ctx,
				c.c,
				c.scheme,

				configName,
				namespace,

				latest.Release,
				cfg,
			)
			_, _ = w.Write([]byte("ok"))
			return
		}

		if r.Method == "GET" {
			latest, _ := config.GetFromConfigMap(c.ctx, c.c, configName, namespace)
			if latest == nil {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(" \"error\": \"no config found\" }"))
				return
			}

			js, err := json.Marshal(latest.Config)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("{ \"error\": \"internal server error\"}"))
				return
			}

			w.Write([]byte(js))
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(" \"error\": \"unsupported request type\" }"))
	}
}
