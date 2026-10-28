// Package ssotest runs an OpenID Connect provider for tests: its configuration and
// keys, a token endpoint that checks the client, redirect and PKCE verifier, and
// user info.
package ssotest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
)

const keyID = "test-key"

// Provider is a fake OpenID Connect provider.
type Provider struct {
	Server       *httptest.Server
	ClientID     string
	ClientSecret string
	// Account is who signs in next: the claims of the ID token besides iss, aud,
	// exp, iat and nonce, such as sub and email.
	Account map[string]any
	// UserInfo is what the userinfo endpoint answers; nil for no claims.
	UserInfo map[string]any
	// Nonce, if set, replaces the nonce of the ID token, as if it came from
	// another sign-in.
	Nonce string

	key  *rsa.PrivateKey
	keys *oidctest.Server

	mu    sync.Mutex
	codes map[string]grant
}

// grant is a code the provider handed out, with what it was asked for.
type grant struct {
	nonce, challenge, redirectURI string
	account                       map[string]any
}

// New starts a provider, closed when the test ends.
func New(t *testing.T) *Provider {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	p := &Provider{
		ClientID:     "parlance",
		ClientSecret: "client-secret",
		key:          key,
		codes:        map[string]grant{},
		keys: &oidctest.Server{
			PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: keyID, Algorithm: oidc.RS256}},
		},
	}
	p.Server = httptest.NewServer(p)
	p.keys.SetIssuer(p.Server.URL)
	t.Cleanup(p.Server.Close)

	return p
}

// Issuer is the provider's issuer URL.
func (p *Provider) Issuer() string {
	return p.Server.URL
}

// SignIn plays someone signing in as Account at the page Parlance sent them to,
// and returns the state and code the provider sends them back with.
func (p *Provider) SignIn(t *testing.T, authorizationURL string) (state, code string) {
	t.Helper()

	parsed, err := url.Parse(authorizationURL)
	if err != nil {
		t.Fatalf("authorization url: %v", err)
	}

	query := parsed.Query()
	if query.Get("client_id") != p.ClientID || query.Get("response_type") != "code" ||
		query.Get("code_challenge_method") != "S256" || !strings.Contains(query.Get("scope"), "openid") {
		t.Fatalf("authorization url %s doesn't ask for a code with PKCE", authorizationURL)
	}

	code = rand.Text()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.codes[code] = grant{
		nonce:       query.Get("nonce"),
		challenge:   query.Get("code_challenge"),
		redirectURI: query.Get("redirect_uri"),
		account:     maps.Clone(p.Account),
	}

	return query.Get("state"), code
}

// ServeHTTP answers the provider's endpoints.
func (p *Provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer":                                p.Issuer(),
			"authorization_endpoint":                p.Issuer() + "/auth",
			"token_endpoint":                        p.Issuer() + "/token",
			"userinfo_endpoint":                     p.Issuer() + "/userinfo",
			"jwks_uri":                              p.Issuer() + "/keys",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{oidc.RS256},
		})
	case "/keys":
		p.keys.ServeHTTP(w, r)
	case "/token":
		p.token(w, r)
	case "/userinfo":
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "no access token", http.StatusUnauthorized)

			return
		}

		info := p.UserInfo
		if info == nil {
			info = map[string]any{}
		}

		writeJSON(w, http.StatusOK, info)
	default:
		http.NotFound(w, r)
	}
}

// token exchanges a code for tokens, as a provider would after checking it.
func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})

		return
	}

	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID, clientSecret = r.Form.Get("client_id"), r.Form.Get("client_secret")
	}

	if clientID != p.ClientID || clientSecret != p.ClientSecret {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})

		return
	}

	p.mu.Lock()
	issued, ok := p.codes[r.Form.Get("code")]
	delete(p.codes, r.Form.Get("code"))
	p.mu.Unlock()

	verifier := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
	if !ok || issued.redirectURI != r.Form.Get("redirect_uri") ||
		base64.RawURLEncoding.EncodeToString(verifier[:]) != issued.challenge {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})

		return
	}

	claims := map[string]any{
		"iss":   p.Issuer(),
		"aud":   p.ClientID,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
		"nonce": issued.nonce,
	}
	if p.Nonce != "" {
		claims["nonce"] = p.Nonce
	}

	maps.Copy(claims, issued.account)

	encoded, err := json.Marshal(claims)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": "access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     oidctest.SignIDToken(p.key, keyID, oidc.RS256, string(encoded)),
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
