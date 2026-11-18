// Package page owns keyset paging for every list the API returns: the opaque
// cursor a client hands back, the clamp on how many rows it may ask for, and
// the probe row that says whether another page exists. See docs/graphql.md.
package page

import (
	"encoding/base64"
	"errors"
	"io"
	"math"
	"strconv"

	"starterkit/internal/confread"
)

var ErrInvalidCursor = errors.New("invalid cursor: pass back an endCursor this API returned")

type Cursor struct {
	id int64
}

func At(id int64) Cursor {
	return Cursor{id: id}
}

func (c Cursor) MarshalGQL(w io.Writer) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(c.id, 10)))

	_, _ = io.WriteString(w, strconv.Quote(encoded))
}

func (c *Cursor) UnmarshalGQL(v any) error {
	text, ok := v.(string)
	if !ok {
		return ErrInvalidCursor
	}

	raw, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil {
		return ErrInvalidCursor
	}

	id, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || id <= 0 {
		return ErrInvalidCursor
	}

	c.id = id

	return nil
}

type Args struct {
	First *int64
	After *Cursor
}

type Limits struct {
	Default int64
	Max     int64
}

func DefaultLimits() Limits {
	return Limits{Default: 50, Max: 500}
}

func (l *Limits) Register(r *confread.Reader) {
	*l = DefaultLimits()

	r.Func("page-default", strconv.FormatInt(l.Default, 10), "rows a list returns when the caller does not say how many", positive(&l.Default))
	r.Func("page-max", strconv.FormatInt(l.Max, 10), "the most rows a list returns however many the caller asks for", positive(&l.Max))
}

func (l *Limits) Check(r *confread.Reader) {
	r.Require("page-max", l.Max >= l.Default, "must be at least page-default")
}

func positive(p *int64) func(raw string) error {
	return func(raw string) error {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return errors.New("must be a positive whole number")
		}

		*p = value

		return nil
	}
}

type Window struct {
	Before int64
	Limit  int64
}

func (l Limits) Window(args Args) Window {
	window := Window{Before: math.MaxInt64, Limit: l.Default}

	if args.After != nil {
		window.Before = args.After.id
	}

	if args.First != nil && *args.First > 0 {
		window.Limit = min(*args.First, l.Max)
	}

	return window
}

func (w Window) Probe() int64 {
	return w.Limit + 1
}

type Info struct {
	HasNextPage bool
	EndCursor   *Cursor
}

func Cut[Node any](window Window, rows []Node, id func(Node) int64) ([]Node, *Info) {
	info := &Info{}

	if int64(len(rows)) > window.Limit {
		rows = rows[:window.Limit]
		info.HasNextPage = true
	}

	if len(rows) > 0 {
		end := At(id(rows[len(rows)-1]))
		info.EndCursor = &end
	}

	return rows, info
}
