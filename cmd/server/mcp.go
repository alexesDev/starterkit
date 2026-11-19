package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"starterkit/internal/appreq"
	"starterkit/internal/audit"
	"starterkit/internal/case/authenticatemcpkey"
	"starterkit/internal/mcpkey"
	"starterkit/internal/mcpserver"
)

const actionMCPPrefix = "mcp:"

func (a *app) mcpRoute() http.Handler {
	bearerOptions := &auth.RequireBearerTokenOptions{AllowMissingExpiration: true}
	requireKey := auth.RequireBearerToken(a.verifyMCPKey, bearerOptions)
	mcpConfig := a.config.MCP
	mcpConfig.Version = mcpserver.Version(a.build.Revision())

	tools := mcpserver.NewHandler(a, mcpConfig)

	return a.withMCPRequest(requireKey(limitBody(tools)))
}

func (a *app) withMCPRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, scoped := a.newRequest(r)

		next.ServeHTTP(w, scoped)

		a.flushAudit(scoped.Context(), req.TakeAudit())
	})
}

func (a *app) verifyMCPKey(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	req, err := appreq.FromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find the request: %w", err)
	}

	authInput := authenticatemcpkey.Input{Secret: mcpkey.Secret(token), IP: req.IP()}

	result, err := authenticatemcpkey.Resolve(ctx, a, authInput)
	if errors.Is(err, authenticatemcpkey.ErrRefused) {
		return nil, fmt.Errorf("%w: %w", auth.ErrInvalidToken, err)
	}

	if err != nil {
		return nil, err
	}

	req.SetIdentity(&result.User, result.IsAdmin)
	req.SetMCPKey(result.KeyID)

	return &auth.TokenInfo{UserID: strconv.FormatInt(result.User.ID, 10)}, nil
}

func (a *app) GenerateMCPKey() (mcpkey.Secret, error) {
	return mcpkey.Generate()
}

type mcpToolDetail struct {
	MCPKeyID *int64 `json:"mcpKeyId"`
	Query    string `json:"query,omitempty"`
}

func (a *app) RecordMCPToolCall(ctx context.Context, call mcpserver.ToolCall) {
	req, err := appreq.FromCtx(ctx)
	if err != nil {
		return
	}

	detail, err := json.Marshal(mcpToolDetail{MCPKeyID: req.MCPKeyID(), Query: call.Query})
	if err != nil {
		a.log.Error("failed to encode an MCP audit detail", "tool", call.Tool, "error", err)
		return
	}

	entry := audit.Entry{
		UserID: req.UserID(),
		Action: actionMCPPrefix + call.Tool,
		Detail: string(detail),
		IP:     req.IP(),
	}

	if user := req.User(); user != nil {
		entry.Email = user.Email
	}

	a.WriteAudit(ctx, entry)
}
