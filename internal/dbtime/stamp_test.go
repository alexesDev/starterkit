package dbtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/dbtime"
)

func TestAtNormalisesToUTC(t *testing.T) {
	zone := time.FixedZone("UTC+3", 3*60*60)
	moment := time.Date(2026, time.August, 10, 12, 30, 0, 0, zone)

	assert.Equal(t, "2026-08-10T09:30:00Z", dbtime.At(moment).Format(time.RFC3339))
}

func TestValueIsUnixSeconds(t *testing.T) {
	moment := dbtime.At(time.Date(2026, time.August, 10, 9, 30, 0, 0, time.UTC))

	value, err := moment.Value()
	require.NoError(t, err)

	assert.Equal(t, moment.Unix(), value)
}

func TestScanReadsUnixSeconds(t *testing.T) {
	var stamp dbtime.Stamp

	require.NoError(t, stamp.Scan(int64(1786000000)))
	assert.Equal(t, time.Unix(1786000000, 0).UTC(), stamp.Time)
}

func TestScanReadsNullAsTheZeroStamp(t *testing.T) {
	stamp := dbtime.At(time.Now())

	require.NoError(t, stamp.Scan(nil))
	assert.True(t, stamp.IsZero())
}

func TestScanRefusesAnythingElse(t *testing.T) {
	var stamp dbtime.Stamp

	require.Error(t, stamp.Scan("2026-08-10 09:30:00"),
		"a text datetime must not be accepted: the column type would be wrong")
}

func TestAStampRoundTripsThroughItsOwnValue(t *testing.T) {
	original := dbtime.At(time.Date(2026, time.August, 10, 9, 30, 0, 0, time.UTC))

	value, err := original.Value()
	require.NoError(t, err)

	var back dbtime.Stamp
	require.NoError(t, back.Scan(value))

	assert.True(t, original.Equal(back.Time))
}

func TestFormattedTakesAGoReferenceLayout(t *testing.T) {
	moment := dbtime.At(time.Date(2026, time.August, 10, 9, 30, 15, 0, time.UTC))

	assert.Equal(t, "2026-08-10 09:30", moment.Formatted("2006-01-02 15:04"))
}

func TestTheSchemaDefaultLayoutIsRFC3339(t *testing.T) {
	moment := dbtime.At(time.Date(2026, time.August, 10, 9, 30, 15, 0, time.UTC))

	assert.Equal(t, "2026-08-10T09:30:15Z", moment.Formatted("2006-01-02T15:04:05Z07:00"))
}
