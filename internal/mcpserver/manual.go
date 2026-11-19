package mcpserver

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"starterkit/internal/textsearch"
)

//go:embed manual/*.md
var manualFiles embed.FS

const (
	indexTopic       = "index"
	maxTopicsPerCall = 5
	maxSearchFiles   = 10
)

var (
	ErrUnknownTopic   = errors.New("unknown topic")
	ErrTooManyTopics  = fmt.Errorf("at most %d topics per call", maxTopicsPerCall)
	ErrTopicsAndQuery = errors.New("pass topics or query, not both")
)

type topic struct {
	name  string
	title string
	lines []string
}

type manual struct {
	topics map[string]topic
	names  []string
}

func loadManual() manual {
	entries, err := fs.ReadDir(manualFiles, "manual")
	if err != nil {
		panic(err)
	}

	loaded := manual{topics: map[string]topic{}}

	for _, entry := range entries {
		raw, readErr := manualFiles.ReadFile(path.Join("manual", entry.Name()))
		if readErr != nil {
			panic(readErr)
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		title := strings.TrimPrefix(lines[0], "# ")

		loaded.topics[name] = topic{name: name, title: title, lines: lines}
		loaded.names = append(loaded.names, name)
	}

	slices.Sort(loaded.names)

	return loaded
}

func (m manual) read(names []string) (string, error) {
	if len(names) == 0 {
		names = []string{indexTopic}
	}

	if len(names) > maxTopicsPerCall {
		return "", ErrTooManyTopics
	}

	found := make([]topic, 0, len(names))

	for _, name := range names {
		chosen, ok := m.topics[strings.TrimSuffix(name, ".md")]
		if !ok {
			return "", fmt.Errorf("%w %q; the topics are: %s", ErrUnknownTopic, name, strings.Join(m.names, ", "))
		}

		found = append(found, chosen)
	}

	parts := make([]string, 0, len(found))

	for _, chosen := range found {
		body := strings.Join(chosen.lines, "\n")
		if len(found) > 1 {
			body = "=== " + chosen.name + " ===\n" + body
		}

		parts = append(parts, body)
	}

	return strings.Join(parts, "\n\n"), nil
}

func (m manual) search(matcher textsearch.Matcher) string {
	type result struct {
		topic   topic
		excerpt textsearch.Excerpt
	}

	var results []result

	for _, name := range m.names {
		chosen := m.topics[name]
		excerpt := textsearch.Cut(chosen.lines, matcher, textsearch.Window{Before: 1, After: 1, Hits: 5, FirstLine: 1})

		if excerpt.Hits > 0 {
			results = append(results, result{topic: chosen, excerpt: excerpt})
		}
	}

	if len(results) == 0 {
		return "No topic matches. Call instructions() for the index."
	}

	slices.SortStableFunc(results, func(a, b result) int {
		return b.excerpt.Hits - a.excerpt.Hits
	})

	var out strings.Builder

	for i, found := range results {
		if i == maxSearchFiles {
			fmt.Fprintf(&out, "... %d more topics match; narrow the query.\n", len(results)-maxSearchFiles)

			break
		}

		fmt.Fprintf(&out, "## %s: %s (%d matching lines)\n%s", found.topic.name, found.topic.title, found.excerpt.Hits, found.excerpt.Text)

		if found.excerpt.Omitted > 0 {
			fmt.Fprintf(&out, "... %d more in this topic\n", found.excerpt.Omitted)
		}

		out.WriteString("\n")
	}

	return out.String()
}
