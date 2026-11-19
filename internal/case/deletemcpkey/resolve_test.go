package deletemcpkey_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/case/deletemcpkey"
	"starterkit/internal/graph/model"
)

func TestResolveDeletesAnExistingKey(t *testing.T) {
	env := &EnvMock{DBDeleteMCPKeyFunc: func(context.Context, int64) (int64, error) { return 1, nil }}

	result, err := deletemcpkey.Resolve(t.Context(), env, deletemcpkey.Input{MCPKeyID: 5})
	require.NoError(t, err)

	success, ok := result.(*model.DeleteMCPKeyPayload)
	require.True(t, ok, "expected success, got %#v", result)
	require.Equal(t, int64(5), success.MCPKeyID)
}

func TestResolveSaysAnUnknownKeyIsNotFound(t *testing.T) {
	env := &EnvMock{DBDeleteMCPKeyFunc: func(context.Context, int64) (int64, error) { return 0, nil }}

	result, err := deletemcpkey.Resolve(t.Context(), env, deletemcpkey.Input{MCPKeyID: 404})
	require.NoError(t, err, "an unknown key must not be an infrastructure error")

	_, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)
}

func TestResolveRefusesAnInvalidIDWithoutTouchingTheDatabase(t *testing.T) {
	env := &EnvMock{}

	result, err := deletemcpkey.Resolve(t.Context(), env, deletemcpkey.Input{MCPKeyID: 0})
	require.NoError(t, err)

	_, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)
	require.Empty(t, env.DBDeleteMCPKeyCalls())
}
