package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func lines(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()

	var parsed []map[string]any

	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}

		var fields map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &fields))
		parsed = append(parsed, fields)
	}

	return parsed
}

func TestALineIsJSONWithItsFieldsAndTheErrorAsText(t *testing.T) {
	var out bytes.Buffer

	log := build(&out, DefaultConfig()).With("job", "purge-audit-log")
	log.Warn("Error running job", "attempt", 2, "error", errors.New("disk full"))

	got := lines(t, &out)
	require.Len(t, got, 1)
	require.Equal(t, "warn", got[0]["level"])
	require.Equal(t, "Error running job", got[0]["message"])
	require.Equal(t, "purge-audit-log", got[0]["job"])
	require.InDelta(t, 2, got[0]["attempt"], 0)
	require.Equal(t, "disk full", got[0]["error"])
	require.NotEmpty(t, got[0]["time"])
}

func TestALineBelowTheLevelIsDropped(t *testing.T) {
	var out bytes.Buffer

	log := build(&out, Config{Level: zerolog.WarnLevel})
	log.Info("started")
	log.Error("failed")

	got := lines(t, &out)
	require.Len(t, got, 1)
	require.Equal(t, "failed", got[0]["message"])
}
