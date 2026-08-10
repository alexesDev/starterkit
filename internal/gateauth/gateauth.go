// Package gateauth reads the identity the forward-auth gate already
// established.
//
// oauth2-proxy runs the authorization-code flow, so the panel does not. It
// receives the resulting ID token in the Authorization header and verifies it
// against the issuer's JWKS rather than trusting the hop. See docs/auth-dex.md.
package gateauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

var (
	ErrNoToken      = errors.New("no identity from the gate")
	ErrInvalidToken = errors.New("unverifiable identity from the gate")
)

type Config struct {
	Issuer   string
	Audience string
}

type Client struct {
	verifier *oidc.IDTokenVerifier
	issuer   string
}

type Identity struct {
	Issuer   string
	Subject  string
	Email    string
	Name     string
	IssuedAt time.Time
}

func New(ctx context.Context, config Config) (*Client, error) {
	provider, err := oidc.NewProvider(ctx, config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to discover oidc issuer %q: %w", config.Issuer, err)
	}

	return &Client{
		verifier: provider.Verifier(&oidc.Config{ClientID: config.Audience}),
		issuer:   config.Issuer,
	}, nil
}

func (c *Client) Identity(ctx context.Context, r *http.Request) (*Identity, error) {
	raw, err := bearerToken(r)
	if err != nil {
		return nil, err
	}

	token, err := c.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to verify id token: %w", ErrInvalidToken, err)
	}

	var claims struct {
		Email    string `json:"email"`
		Verified *bool  `json:"email_verified"`
		Name     string `json:"name"`
	}

	err = token.Claims(&claims)
	if err != nil {
		return nil, fmt.Errorf("failed to read id token claims: %w", err)
	}

	if claims.Email == "" {
		return nil, errors.New("id token has no email claim")
	}

	if claims.Verified == nil || !*claims.Verified {
		return nil, errors.New("id token email is not verified")
	}

	return &Identity{
		Issuer:   c.issuer,
		Subject:  token.Subject,
		Email:    claims.Email,
		Name:     claims.Name,
		IssuedAt: token.IssuedAt,
	}, nil
}

func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", ErrNoToken
	}

	raw, found := strings.CutPrefix(header, "Bearer ")
	if !found || raw == "" {
		return "", ErrNoToken
	}

	return raw, nil
}
