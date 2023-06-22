package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wandb/operator/pkg/utils/kubeclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Namespace = "wandb"
const PasswordSecret = "wandb-password"
const PasswordKey = "password"

const CookieAuthName = "wandb_console_auth"

func GetPassword(ctx context.Context, c client.Client) (string, error) {
	kubeclient.UpsertNamespace(ctx, c, Namespace)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PasswordSecret,
			Namespace: Namespace,
		},
		Data: map[string][]byte{},
	}
	err := kubeclient.GetOrCreate(ctx, c, secret)
	if err != nil {
		return "", err
	}

	password := secret.Data[PasswordKey]
	return string(password), nil
}

func SetPassword(ctx context.Context, c client.Client, password string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PasswordSecret,
			Namespace: Namespace,
		},
		Data: map[string][]byte{
			PasswordKey: []byte(password),
		},
	}
	return kubeclient.CreateOrUpdate(ctx, c, secret)
}

func IsPasswordSet(ctx context.Context, c client.Client) bool {
	p, err := GetPassword(ctx, c)
	return err == nil && p != ""
}

func isPassword(ctx context.Context, c client.Client, password string) bool {
	p, _ := GetPassword(ctx, c)
	return p == password && p != ""
}

type PasswordMiddleware struct {
	ctx         context.Context
	c           client.Client
	ignorePaths []string
}

func getPassword(r *http.Request) (string, error) {
	cookie, err := r.Cookie(CookieAuthName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func (m PasswordMiddleware) checkAuth(w http.ResponseWriter, r *http.Request) {
	pwd, err := getPassword(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("{ \"error\": \"user not authentication\"}"))
		return
	}

	if !isPassword(m.ctx, m.c, pwd) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("{ \"error\": \"user not authentication\"}"))
		return
	}
}

func (m PasswordMiddleware) Auth(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, path := range m.ignorePaths {
			if strings.HasPrefix(r.URL.Path, path) {
				handler.ServeHTTP(w, r)
				return
			}
		}

		m.checkAuth(w, r)

		handler.ServeHTTP(w, r)
	})
}

func (m PasswordMiddleware) Password() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("{ \"error\": \"method not allowed\"}"))
			return
		}

		if IsPasswordSet(m.ctx, m.c) {
			m.checkAuth(w, r)
		}

		pwdb, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("{ \"error\": \"internal server error\"}"))
			return
		}

		pwd := string(pwdb)
		if len(pwd) < 8 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("{ \"error\": \"password must be at least 8 characters\"}"))
			return
		}

		SetPassword(m.ctx, m.c, string(pwd))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{ \"message\": \"password set\"}"))
	}
}

func (m PasswordMiddleware) Viewer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("{ \"error\": \"method not allowed\"}"))
			return
		}

		pwd, _ := getPassword(r)
		data := map[string]interface{}{
			"isPasswordSet": IsPasswordSet(m.ctx, m.c),
			"loggedIn":      isPassword(m.ctx, m.c, pwd),
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("{ \"error\": \"internal server error\"}"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	}
}

func (m PasswordMiddleware) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("{ \"error\": \"method not allowed\"}"))
			return
		}

		pwdb, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("{ \"error\": \"internal server error\"}"))
			return
		}
		pwd := string(pwdb)

		if !isPassword(m.ctx, m.c, pwd) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("{ \"error\": \"user not authentication\"}"))
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:    CookieAuthName,
			Value:   pwd,
			Path:    "/",
			Secure:  true,
			Expires: time.Now().Add(7 * 24 * time.Hour),
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func (m PasswordMiddleware) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("{ \"error\": \"method not allowed\"}"))
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:   CookieAuthName,
			Value:  "",
			Path:   "/",
			Secure: true,
			MaxAge: -1,
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}
