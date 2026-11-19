// Package textsearch is the grep both MCP lookups share: a query over lines,
// matched case-insensitively by any word, every word, or the exact phrase, and
// the hits cut out with a few lines of context around them.
package textsearch

import (
	"errors"
	"slices"
	"strconv"
	"strings"
)

type Op string

const (
	OpOr    Op = "or"
	OpAnd   Op = "and"
	OpExact Op = "exact"
)

var (
	ErrEmptyQuery = errors.New("the query is empty")
	ErrUnknownOp  = errors.New(`query_op must be "or", "and" or "exact"`)
)

type Matcher struct {
	op    Op
	terms []string
}

func Compile(query string, op Op) (Matcher, error) {
	if op == "" {
		op = OpAnd
	}

	if !slices.Contains([]Op{OpOr, OpAnd, OpExact}, op) {
		return Matcher{}, ErrUnknownOp
	}

	lowered := strings.ToLower(strings.TrimSpace(query))
	if lowered == "" {
		return Matcher{}, ErrEmptyQuery
	}

	terms := strings.Fields(lowered)
	if op == OpExact {
		terms = []string{lowered}
	}

	return Matcher{op: op, terms: terms}, nil
}

func (m Matcher) Match(line string) bool {
	lowered := strings.ToLower(line)

	contains := func(term string) bool {
		return strings.Contains(lowered, term)
	}

	if m.op == OpOr {
		return slices.ContainsFunc(m.terms, contains)
	}

	for _, term := range m.terms {
		if !contains(term) {
			return false
		}
	}

	return true
}

type Window struct {
	Before    int
	After     int
	Hits      int
	FirstLine int
}

type Excerpt struct {
	Text    string
	Hits    int
	Omitted int
}

func Cut(lines []string, matcher Matcher, window Window) Excerpt {
	var hits []int

	for i, line := range lines {
		if matcher.Match(line) {
			hits = append(hits, i)
		}
	}

	shown := hits[:min(len(hits), window.Hits)]

	var out strings.Builder

	last := -1

	for _, hit := range shown {
		first := max(hit-window.Before, last+1)
		end := min(hit+window.After, len(lines)-1)

		if last >= 0 && first > last+1 {
			out.WriteString("--\n")
		}

		for i := first; i <= end; i++ {
			marker := "-"
			if slices.Contains(hits, i) {
				marker = ":"
			}

			out.WriteString(strconv.Itoa(i+window.FirstLine) + marker + " " + lines[i] + "\n")
		}

		last = end
	}

	return Excerpt{Text: out.String(), Hits: len(hits), Omitted: len(hits) - len(shown)}
}
