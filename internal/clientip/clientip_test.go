package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/clientip"
)

func localhost() []netip.Prefix {
	trusted, _ := clientip.Parse("127.0.0.1/32,::1/128")

	return trusted
}

func request(remote string, forwarded ...string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote

	for _, value := range forwarded {
		r.Header.Add("X-Forwarded-For", value)
	}

	return r
}

func TestDirectRequestUsesTheSocketAddress(t *testing.T) {
	got := clientip.Resolve(request("203.0.113.9:5555"), localhost())
	require.Equal(t, "203.0.113.9", got)
}

func TestForwardedHeaderIsIgnoredFromAnUntrustedPeer(t *testing.T) {
	got := clientip.Resolve(request("203.0.113.9:5555", "10.0.0.1"), localhost())
	require.Equal(t, "203.0.113.9", got, "spoofed header was trusted: got %q", got)
}

func TestForwardedHeaderIsUsedFromATrustedProxy(t *testing.T) {
	got := clientip.Resolve(request("127.0.0.1:5555", "198.51.100.7"), localhost())
	require.Equal(t, "198.51.100.7", got)
}

func TestTheRightmostUntrustedHopWins(t *testing.T) {
	got := clientip.Resolve(request("127.0.0.1:5555", "1.2.3.4, 198.51.100.7"), localhost())
	require.Equal(t, "198.51.100.7", got)
}

func TestTrustedHopsAreSkipped(t *testing.T) {
	trusted, err := clientip.Parse("127.0.0.1/32,10.0.0.0/8")
	require.NoError(t, err)

	got := clientip.Resolve(request("127.0.0.1:5555", "198.51.100.7, 10.0.0.5"), trusted)
	require.Equal(t, "198.51.100.7", got)
}

func TestMultipleHeadersAreJoined(t *testing.T) {
	got := clientip.Resolve(request("127.0.0.1:5555", "198.51.100.7", "10.0.0.5"), localhost())
	require.Equal(t, "10.0.0.5", got)
}

func TestGarbageInTheHeaderFallsBackToThePeer(t *testing.T) {
	got := clientip.Resolve(request("127.0.0.1:5555", "not-an-address"), localhost())
	require.Equal(t, "127.0.0.1", got)
}

func TestNoTrustedProxiesMeansTheHeaderIsNeverUsed(t *testing.T) {
	got := clientip.Resolve(request("127.0.0.1:5555", "198.51.100.7"), nil)
	require.Equal(t, "127.0.0.1", got)
}

func TestParseRefusesAMalformedEntryByPosition(t *testing.T) {
	_, err := clientip.Parse("127.0.0.1/32, nonsense, 10.0.0.0/8")
	require.EqualError(t, err, "entry 2 is not a CIDR such as 10.0.0.0/8")
}

func TestParseSkipsEmptyEntries(t *testing.T) {
	got, err := clientip.Parse("127.0.0.1/32,, 10.0.0.0/8 ")
	require.NoError(t, err)
	require.Len(t, got, 2)
}
