package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

type Route struct {
	Prefix  string
	Target  string
	Public  bool
}

var routes []Route

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func init() {
	authServiceURL := getEnv("AUTH_SERVICE_URL", "http://auth-service")

	routes = []Route{
		{Prefix: "/auth/", Target: authServiceURL, Public: true},
		{Prefix: "/api/", Target: "", Public: false},
	}
}

func validateToken(token string) (map[string]any, error) {
	authServiceURL := getEnv("AUTH_SERVICE_URL", "http://auth-service")
	req, err := http.NewRequest("GET", authServiceURL+"/validate", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid token: %s", string(body))
	}

	var result struct {
		Valid  bool           `json:"valid"`
		Claims map[string]any `json:"claims"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Claims, nil
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if cookie, err := r.Cookie("access_token"); err == nil {
		return cookie.Value
	}
	return ""
}

func newReverseProxy(target string) *httputil.ReverseProxy {
	u, _ := url.Parse(target)
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error for %s: %v", r.URL.Path, err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}
	return proxy
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "api-gateway"})
}

func gatewayHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", getEnv("ALLOWED_ORIGIN", "http://localhost:3000"))
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, route := range routes {
		if !strings.HasPrefix(path, route.Prefix) {
			continue
		}

		if !route.Public {
			token := extractToken(r)
			if token == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			claims, err := validateToken(token)
			if err != nil {
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}
			// Forward user info to downstream services
			if sub, ok := claims["sub"].(string); ok {
				r.Header.Set("X-User-ID", sub)
			}
			if email, ok := claims["email"].(string); ok {
				r.Header.Set("X-User-Email", email)
			}
		}

		if route.Target == "" {
			http.Error(w, "Service not configured", http.StatusNotFound)
			return
		}

		proxy := newReverseProxy(route.Target)
		// Strip prefix before proxying
		r.URL.Path = strings.TrimPrefix(path, route.Prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		proxy.ServeHTTP(w, r)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	claims, err := validateToken(token)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(claims)
}

func main() {
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/api/me", handleMe)
	http.HandleFunc("/api/health", handleHealth)
	http.HandleFunc("/", gatewayHandler)

	port := getEnv("PORT", "8080")
	log.Printf("API Gateway starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
