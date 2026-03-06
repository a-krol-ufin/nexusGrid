package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
	ready        atomic.Bool
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type oidcDiscovery struct {
	Issuer   string `json:"issuer"`
	AuthURL  string `json:"authorization_endpoint"`
	TokenURL string `json:"token_endpoint"`
	JWKSURI  string `json:"jwks_uri"`
}

func fetchDiscovery(url string) (*oidcDiscovery, error) {
	resp, err := http.Get(url + "/.well-known/openid-configuration")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var d oidcDiscovery
	return &d, json.NewDecoder(resp.Body).Decode(&d)
}

func initKeycloak() {
	internalURL := getEnv("KEYCLOAK_INTERNAL_URL", "http://keycloak:8080")
	publicURL := getEnv("KEYCLOAK_PUBLIC_URL", "http://localhost:8180")
	realm := getEnv("KEYCLOAK_REALM", "nexusgrid")
	clientID := getEnv("KEYCLOAK_CLIENT_ID", "nexusgrid-client")
	clientSecret := getEnv("KEYCLOAK_CLIENT_SECRET", "")
	redirectURL := getEnv("REDIRECT_URL", "http://localhost:8081/callback")

	internalIssuer := fmt.Sprintf("%s/realms/%s", internalURL, realm)
	publicIssuer := fmt.Sprintf("%s/realms/%s", publicURL, realm)

	// Retry until Keycloak is up
	var discovery *oidcDiscovery
	var err error
	for i := 1; ; i++ {
		discovery, err = fetchDiscovery(internalIssuer)
		if err == nil {
			break
		}
		log.Printf("Waiting for Keycloak (attempt %d): %v", i, err)
		time.Sleep(5 * time.Second)
	}

	// Rewrite any public-facing URLs in the discovery doc to internal ones
	// so server-side calls (JWKS fetch, token exchange) go via cluster DNS
	jwksURI := strings.ReplaceAll(discovery.JWKSURI, publicURL, internalURL)
	tokenURL := strings.ReplaceAll(discovery.TokenURL, publicURL, internalURL)

	// Build verifier directly — skip oidc.NewProvider which enforces issuer match
	keySet := oidc.NewRemoteKeySet(context.Background(), jwksURI)
	verifier = oidc.NewVerifier(publicIssuer, keySet, &oidc.Config{
		ClientID: clientID,
	})

	oauth2Config = oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			// AuthURL is browser-facing → use public URL
			AuthURL: publicIssuer + "/protocol/openid-connect/auth",
			// TokenURL is server-side → use internal URL
			TokenURL: tokenURL,
		},
		Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
	}

	ready.Store(true)
	log.Println("Keycloak connection established — auth service ready")
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// /healthz — liveness: is the process alive? Always 200.
func handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

// /readyz — readiness: is Keycloak connected? 503 until ready.
func handleReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "waiting for keycloak"})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func requireReady(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			http.Error(w, "Service initializing, please retry shortly", http.StatusServiceUnavailable)
			return
		}
		next(w, r)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		HttpOnly: true,
		Path:     "/",
		MaxAge:   300,
	})
	http.Redirect(w, r, oauth2Config.AuthCodeURL(state), http.StatusFound)
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	token, err := oauth2Config.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "Token exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token in response", http.StatusInternalServerError)
		return
	}

	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "ID token verification failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    token.AccessToken,
		HttpOnly: true,
		Path:     "/",
		MaxAge:   int(time.Until(token.Expiry).Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")
	http.Redirect(w, r, frontendURL+"/dashboard", http.StatusFound)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "access_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	publicURL := getEnv("KEYCLOAK_PUBLIC_URL", "http://localhost:8180")
	realm := getEnv("KEYCLOAK_REALM", "nexusgrid")
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")
	logoutURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/logout?redirect_uri=%s", publicURL, realm, frontendURL)
	http.Redirect(w, r, logoutURL, http.StatusFound)
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.Header.Get("Authorization")
	if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
		tokenStr = tokenStr[7:]
	} else {
		http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}

	idToken, err := verifier.Verify(r.Context(), tokenStr)
	if err != nil {
		http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"valid":  true,
		"claims": claims,
	})
}

func main() {
	go initKeycloak()

	http.HandleFunc("/healthz", handleLiveness)
	http.HandleFunc("/readyz", handleReadiness)
	http.HandleFunc("/login", requireReady(handleLogin))
	http.HandleFunc("/callback", requireReady(handleCallback))
	http.HandleFunc("/logout", requireReady(handleLogout))
	http.HandleFunc("/validate", requireReady(handleValidate))

	port := getEnv("PORT", "8080")
	log.Printf("Auth service starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
