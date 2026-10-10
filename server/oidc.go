package server

import (
	"cmp"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OIDC signs people in through an identity provider (Keycloak, GitLab, Google, Okta) with the
// authorization code flow and PKCE. The CLI opens /v1/oidc/start?state=S in a browser and polls
// /v1/oidc/poll for the session token the callback leaves under S.
type OIDC struct {
	Issuer       string `yaml:"issuer" json:"issuer"`
	ClientID     string `yaml:"client_id" json:"client_id"`
	ClientSecret string `yaml:"client_secret" json:"client_secret"`
	// RedirectURL is this server's /v1/oidc/callback as the provider reaches it.
	RedirectURL string `yaml:"redirect_url" json:"redirect_url"`
	// UserClaim names the account (default preferred_username, then email).
	UserClaim   string   `yaml:"user_claim" json:"user_claim"`
	GroupsClaim string   `yaml:"groups_claim" json:"groups_claim"`
	Roles       []string `yaml:"roles" json:"roles"`

	client *http.Client
	mu     sync.Mutex
	meta   *oidcMeta
	keys   map[string]*rsa.PublicKey
	flows  map[string]*oidcFlow
}

type oidcMeta struct {
	Issuer   string `json:"issuer"`
	AuthURL  string `json:"authorization_endpoint"`
	TokenURL string `json:"token_endpoint"`
	JWKSURL  string `json:"jwks_uri"`
}

type oidcFlow struct {
	nonce, verifier string
	started         time.Time
	token           string
	err             string
}

func (o *OIDC) http() *http.Client {
	if o.client != nil {
		return o.client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (o *OIDC) discover(ctx context.Context) (*oidcMeta, error) {
	o.mu.Lock()
	m := o.meta
	o.mu.Unlock()
	if m != nil {
		return m, nil
	}
	m = &oidcMeta{}
	if err := o.getJSON(ctx, strings.TrimSuffix(o.Issuer, "/")+"/.well-known/openid-configuration", m); err != nil {
		return nil, err
	}
	if m.Issuer != o.Issuer {
		return nil, fmt.Errorf("oidc: issuer %q, configured %q", m.Issuer, o.Issuer)
	}
	o.mu.Lock()
	o.meta = m
	o.mu.Unlock()
	return m, nil
}

func (o *OIDC) getJSON(ctx context.Context, u string, v any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := o.http().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("oidc: %s: %s", u, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// start begins a flow under state and returns where to send the browser.
func (o *OIDC) start(ctx context.Context, state string) (string, error) {
	if len(state) < 16 {
		return "", errors.New("state too short")
	}
	m, err := o.discover(ctx)
	if err != nil {
		return "", err
	}
	f := &oidcFlow{nonce: randomString(16), verifier: randomString(32), started: time.Now()}
	o.mu.Lock()
	if o.flows == nil {
		o.flows = map[string]*oidcFlow{}
	}
	for s, old := range o.flows {
		if time.Since(old.started) > 10*time.Minute {
			delete(o.flows, s)
		}
	}
	if _, taken := o.flows[state]; taken {
		o.mu.Unlock()
		return "", errors.New("state in use")
	}
	o.flows[state] = f
	o.mu.Unlock()
	sum := sha256.Sum256([]byte(f.verifier))
	q := url.Values{"response_type": {"code"}, "client_id": {o.ClientID}, "redirect_uri": {o.RedirectURL},
		"scope": {"openid profile email"}, "state": {state}, "nonce": {f.nonce},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}
	return m.AuthURL + "?" + q.Encode(), nil
}

type oidcIdentity struct {
	User   string
	Groups []string
}

// callback trades code for an ID token and checks it.
func (o *OIDC) callback(ctx context.Context, state, code string) (*oidcFlow, oidcIdentity, error) {
	o.mu.Lock()
	f := o.flows[state]
	o.mu.Unlock()
	if f == nil {
		return nil, oidcIdentity{}, errors.New("unknown or expired sign-in")
	}
	m, err := o.discover(ctx)
	if err != nil {
		return f, oidcIdentity{}, err
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {o.RedirectURL},
		"client_id": {o.ClientID}, "client_secret": {o.ClientSecret}, "code_verifier": {f.verifier}}
	req, _ := http.NewRequestWithContext(ctx, "POST", m.TokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.http().Do(req)
	if err != nil {
		return f, oidcIdentity{}, err
	}
	defer resp.Body.Close()
	var tok struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.IDToken == "" {
		return f, oidcIdentity{}, fmt.Errorf("oidc: token exchange failed: %s %s", resp.Status, tok.Error)
	}
	claims, err := o.verify(ctx, tok.IDToken)
	if err != nil {
		return f, oidcIdentity{}, err
	}
	if claims["nonce"] != f.nonce {
		return f, oidcIdentity{}, errors.New("oidc: nonce mismatch")
	}
	var id oidcIdentity
	for _, c := range []string{o.UserClaim, "preferred_username", "email"} {
		if s, _ := claims[c].(string); c != "" && s != "" {
			id.User = s
			break
		}
	}
	if id.User == "" {
		return f, id, errors.New("oidc: the ID token names no user")
	}
	if gs, ok := claims[cmp.Or(o.GroupsClaim, "groups")].([]any); ok {
		for _, g := range gs {
			if s, ok := g.(string); ok {
				id.Groups = append(id.Groups, strings.TrimPrefix(s, "/"))
			}
		}
	}
	return f, id, nil
}

// verify checks an RS256 ID token's signature, issuer, audience and expiry, and returns its claims.
func (o *OIDC) verify(ctx context.Context, raw string) (map[string]any, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("oidc: malformed ID token")
	}
	var head struct{ Alg, Kid string }
	if err := decodeSegment(parts[0], &head); err != nil {
		return nil, err
	}
	if head.Alg != "RS256" {
		return nil, fmt.Errorf("oidc: unsupported alg %s", head.Alg)
	}
	key, err := o.key(ctx, head.Kid)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return nil, errors.New("oidc: bad ID token signature")
	}
	var claims map[string]any
	if err := decodeSegment(parts[1], &claims); err != nil {
		return nil, err
	}
	if claims["iss"] != o.Issuer {
		return nil, errors.New("oidc: wrong issuer")
	}
	aud := claims["aud"]
	if auds, ok := aud.([]any); ok {
		aud = nil
		for _, a := range auds {
			if a == o.ClientID {
				aud = a
			}
		}
	}
	if aud != o.ClientID {
		return nil, errors.New("oidc: token is for another client")
	}
	if exp, _ := claims["exp"].(float64); time.Now().After(time.Unix(int64(exp), 0)) {
		return nil, errors.New("oidc: ID token expired")
	}
	return claims, nil
}

// key finds kid in the provider's JWKS, refetching once for a key it has not seen (rotation).
func (o *OIDC) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	o.mu.Lock()
	k := o.keys[kid]
	o.mu.Unlock()
	if k != nil {
		return k, nil
	}
	m, err := o.discover(ctx)
	if err != nil {
		return nil, err
	}
	var set struct {
		Keys []struct{ Kty, Kid, N, E string }
	}
	if err := o.getJSON(ctx, m.JWKSURL, &set); err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, j := range set.Keys {
		n, err1 := base64.RawURLEncoding.DecodeString(j.N)
		e, err2 := base64.RawURLEncoding.DecodeString(j.E)
		if j.Kty != "RSA" || err1 != nil || err2 != nil {
			continue
		}
		keys[j.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	o.mu.Lock()
	o.keys = keys
	o.mu.Unlock()
	if k = keys[kid]; k == nil {
		return nil, fmt.Errorf("oidc: no key %q", kid)
	}
	return k, nil
}

func (o *OIDC) finish(state, token, errText string) {
	o.mu.Lock()
	if f := o.flows[state]; f != nil {
		f.token, f.err = token, errText
	}
	o.mu.Unlock()
}

// poll hands out a finished flow's token once.
func (o *OIDC) poll(state string) (token string, done bool, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	f := o.flows[state]
	if f == nil {
		return "", true, errors.New("unknown or expired sign-in")
	}
	if f.token == "" && f.err == "" {
		return "", false, nil
	}
	delete(o.flows, state)
	if f.err != "" {
		return "", true, errors.New(f.err)
	}
	return f.token, true, nil
}

func decodeSegment(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
