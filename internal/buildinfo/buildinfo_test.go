package buildinfo_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"starterkit/internal/buildinfo"
)

const commit = "0123456789abcdef0123456789abcdef01234567"

func TestAStampedBuildNamesItsCommitTimeAndLink(t *testing.T) {
	info, err := buildinfo.New(commit, "2026-11-18T09:30:15Z", "https://git.example.com/org/app")
	require.NoError(t, err)

	require.Equal(t, commit, *info.Commit())
	require.Equal(t, time.Date(2026, 11, 18, 9, 30, 15, 0, time.UTC), info.BuiltAt().Time)
	require.Equal(t, "https://git.example.com/org/app/commit/"+commit, *info.CommitURL())
	require.Equal(t, commit, info.Revision())
}

func TestADevBuildClaimsNoCommit(t *testing.T) {
	info, err := buildinfo.New("", "", "https://git.example.com/org/app")
	require.NoError(t, err)

	require.Nil(t, info.Commit())
	require.Nil(t, info.BuiltAt())
	require.Nil(t, info.CommitURL())
	require.Equal(t, "dev", info.Revision())
}

func TestNoRepositoryMeansNoLink(t *testing.T) {
	info, err := buildinfo.New(commit, "", "")
	require.NoError(t, err)

	require.Nil(t, info.CommitURL())
}

func TestABuildTimeThatIsNotRFC3339RefusesToStart(t *testing.T) {
	_, err := buildinfo.New(commit, "18.11.2026", "")
	require.ErrorContains(t, err, "not RFC 3339")
}

func TestEveryResponseCarriesTheCommit(t *testing.T) {
	info, err := buildinfo.New(commit, "", "")
	require.NoError(t, err)

	w := serve(info)

	require.Equal(t, commit, w.Header().Get(buildinfo.CommitHeader))
	require.Equal(t, http.StatusTeapot, w.Code)
}

func TestADevBuildSendsNoCommitHeader(t *testing.T) {
	info, err := buildinfo.New("", "", "")
	require.NoError(t, err)

	w := serve(info)

	require.Empty(t, w.Header().Values(buildinfo.CommitHeader))
	require.Equal(t, http.StatusTeapot, w.Code)
}

func serve(info buildinfo.Info) *httptest.ResponseRecorder {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	w := httptest.NewRecorder()
	info.Announce(inner).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/graphql", nil))

	return w
}
