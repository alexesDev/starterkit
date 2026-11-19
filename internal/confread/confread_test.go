package confread_test

import (
	"bytes"
	"flag"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"starterkit/internal/confread"
)

type settings struct {
	addr     string
	dev      bool
	interval time.Duration
	keep     int
}

func read(t *testing.T, args, environ []string) (settings, error) {
	t.Helper()

	got := settings{addr: "127.0.0.1:1", interval: time.Minute, keep: 7}
	r := confread.New("test", "KIT_", environ)
	r.SetOutput(&bytes.Buffer{})

	r.String(&got.addr, "listen-addr", "where to listen")
	r.Bool(&got.dev, "dev-mode", "development mode")
	r.Duration(&got.interval, "poll-interval", time.Second, "how often to poll")
	r.Int(&got.keep, "keep", 0, "how many to keep")

	r.Parse(args)
	r.Require("listen-addr", got.addr != "", "is empty")

	return got, r.Err()
}

func TestDefaultsStandWithNothingSet(t *testing.T) {
	got, err := read(t, nil, []string{"HOME=/root"})
	require.NoError(t, err)
	require.Equal(t, settings{addr: "127.0.0.1:1", interval: time.Minute, keep: 7}, got)
}

func TestTheEnvironmentNameIsDerivedFromTheFlag(t *testing.T) {
	environ := []string{"KIT_LISTEN_ADDR=0.0.0.0:2", "KIT_DEV_MODE=true", "KIT_POLL_INTERVAL=5s", "KIT_KEEP=3"}

	got, err := read(t, nil, environ)
	require.NoError(t, err)
	require.Equal(t, settings{addr: "0.0.0.0:2", dev: true, interval: 5 * time.Second, keep: 3}, got)
}

func TestAFlagWinsOverTheEnvironment(t *testing.T) {
	got, err := read(t, []string{"--listen-addr=10.0.0.1:3"}, []string{"KIT_LISTEN_ADDR=0.0.0.0:2"})
	require.NoError(t, err)
	require.Equal(t, "10.0.0.1:3", got.addr)
}

func TestAnEmptyVariableIsNotApplied(t *testing.T) {
	got, err := read(t, nil, []string{"KIT_LISTEN_ADDR="})
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:1", got.addr)
}

func TestEveryProblemIsReportedAtOnce(t *testing.T) {
	environ := []string{"KIT_DEV_MODE=sometimes", "KIT_POLL_INTERVAL=10", "KIT_LISTEN_ADRR=0.0.0.0:2"}

	_, err := read(t, []string{"--listen-addr="}, environ)
	require.Error(t, err)

	require.Contains(t, err.Error(), "KIT_DEV_MODE: must be true or false")
	require.Contains(t, err.Error(), "KIT_POLL_INTERVAL: must be a duration with a unit")
	require.Contains(t, err.Error(), "KIT_LISTEN_ADRR names no setting")
	require.Contains(t, err.Error(), "KIT_LISTEN_ADDR (--listen-addr): is empty")
}

func TestABadFlagIsCollectedRatherThanStoppingTheParse(t *testing.T) {
	_, err := read(t, []string{"--poll-interval=1ms", "--dev-mode=maybe", "--keep=-1"}, nil)
	require.Error(t, err)

	require.Contains(t, err.Error(), "--keep: must be a whole number of at least 0")
	require.Contains(t, err.Error(), "--poll-interval: must be a duration with a unit, such as 30s, of at least 1s")
	require.Contains(t, err.Error(), "--dev-mode: must be true or false")
}

func TestAProblemNeverEchoesTheValue(t *testing.T) {
	const secret = "hunter2-hunter2"

	_, err := read(t, []string{"--dev-mode=" + secret}, []string{"KIT_POLL_INTERVAL=" + secret, "KIT_TYPO=" + secret})
	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
}

func TestHelpPrintsTheDefaultsAndTheKeysButNotTheEnvironment(t *testing.T) {
	var out bytes.Buffer

	addr := "127.0.0.1:1"
	r := confread.New("test", "KIT_", []string{"KIT_LISTEN_ADDR=10.9.8.7:6"})
	r.SetOutput(&out)
	r.String(&addr, "listen-addr", "where to listen")

	r.Parse([]string{"--help"})

	require.ErrorIs(t, r.Err(), flag.ErrHelp)
	require.Contains(t, out.String(), "[KIT_LISTEN_ADDR] where to listen")
	require.Contains(t, out.String(), "127.0.0.1:1")
	require.NotContains(t, out.String(), "10.9.8.7")
}

func TestAKeyReplacesTheDerivedName(t *testing.T) {
	url := "sqlite:default.db"
	r := confread.New("test", "KIT_", []string{"DATABASE_URL=sqlite:other.db"})
	r.String(&url, "database-url", "database")
	r.Key("database-url", "DATABASE_URL")

	r.Parse(nil)

	require.NoError(t, r.Err())
	require.Equal(t, "sqlite:other.db", url)
}

func TestTheDerivedNameIsUnknownOnceAKeyReplacesIt(t *testing.T) {
	url := "sqlite:default.db"
	r := confread.New("test", "KIT_", []string{"KIT_DATABASE_URL=sqlite:other.db"})
	r.String(&url, "database-url", "database")
	r.Key("database-url", "DATABASE_URL")

	r.Parse(nil)

	require.ErrorContains(t, r.Err(), "KIT_DATABASE_URL names no setting")
	require.Equal(t, "sqlite:default.db", url)
}
