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
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	provider     *oidc.Provider
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func init() {
	// Internal URL: used by this pod to reach Keycloak inside the cluster
	internalURL := getEnv("KEYCLOAK_INTERNAL_URL", "http://keycloak:8080")
	// Public URL: what the browser sees (localhost via port-forward)
	publicURL := getEnv("KEYCLOAK_PUBLIC_URL", "http://localhost:8180")
	realm := getEnv("KEYCLOAK_REALM", "nexusgrid")
	clientID := getEnv("KEYCLOAK_CLIENT_ID", "nexusgrid-client")
	clientSecret := getEnv("KEYCLOAK_CLIENT_SECRET", "")
	redirectURL := getEnv("REDIRECT_URL", "http://localhost:8081/callback")

	// Discover OIDC config via internal URL
	internalIssuer := fmt.Sprintf("%s/realms/%s", internalURL, realm)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var err error
	for i := 0; i < 12; i++ {
		provider, err = oidc.NewProvider(ctx, internalIssuer)
		if err == nil {
			break
		}
		log.Printf("Waiting for Keycloak... attempt %d/12: %v", i+1, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to Keycloak: %v", err)
	}

	// Build endpoint using public URL so browser redirects go to localhost
	publicIssuer := fmt.Sprintf("%s/realms/%s", publicURL, realm)
	endpoint := oauth2.Endpoint{
		AuthURL:  publicIssuer + "/protocol/openid-connect/auth",
		TokenURL: internalIssuer + "/protocol/openid-connect/token",
	}

	oauth2Config = oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     endpoint,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	// SkipIssuerCheck because tokens will have the public issuer URL
	// but we discovered via the internal URL
	verifier = provider.Verifier(&oidc.Config{
		ClientID:        clientID,
		SkipIssuerCheck: true,
	})
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/callback", handleCallback)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/validate", handleValidate)

	port := getEnv("PORT", "8080")
	log.Printf("Auth service starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
