// Package dbtime gives a time column a type.
//
// Timestamps are stored as integer unix seconds in STRICT tables; a sqlc
// `overrides` entry maps every one of them to Stamp, so no layer ever handles a
// bare int64. Stamp embeds time.Time, so Format, Before, Unix and the rest work
// on it directly. See docs/rules.md.
package dbtime

import (
	"database/sql/driver"
	"fmt"
	"time"
)

type Stamp struct {
	time.Time
}

func At(moment time.Time) Stamp {
	return Stamp{Time: moment.UTC()}
}

func (s *Stamp) Scan(src any) error {
	switch value := src.(type) {
	case int64:
		*s = Stamp{Time: time.Unix(value, 0).UTC()}

		return nil
	case nil:
		*s = Stamp{}

		return nil
	}

	return fmt.Errorf("cannot scan %T into a stamp", src)
}

func (s Stamp) Value() (driver.Value, error) {
	return s.Unix(), nil
}

func (s Stamp) Formatted(layout string) string {
	return s.Format(layout)
}
