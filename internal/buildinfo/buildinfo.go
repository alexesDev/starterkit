// Package buildinfo names the build a binary came from: the commit and the time
// the image build stamped into it with -ldflags -X. A binary told no commit is a
// dev build and never claims one. See docs/build-info.md.
package buildinfo

import (
	"fmt"
	"net/http"
	"time"

	"starterkit/internal/dbtime"
)

const (
	CommitHeader = "X-Build-Commit"
	devRevision  = "dev"
)

type Info struct {
	commit    string
	builtAt   *dbtime.Stamp
	commitURL *string
}

func New(commit, builtAt, repoURL string) (Info, error) {
	info := Info{commit: commit}

	if builtAt != "" {
		moment, err := time.Parse(time.RFC3339, builtAt)
		if err != nil {
			return Info{}, fmt.Errorf("build time %q is not RFC 3339: %w", builtAt, err)
		}

		stamp := dbtime.At(moment)
		info.builtAt = &stamp
	}

	if commit != "" && repoURL != "" {
		link := repoURL + "/commit/" + commit
		info.commitURL = &link
	}

	return info, nil
}

func (i Info) Commit() *string {
	if i.commit == "" {
		return nil
	}

	return &i.commit
}

func (i Info) BuiltAt() *dbtime.Stamp {
	return i.builtAt
}

func (i Info) CommitURL() *string {
	return i.commitURL
}

func (i Info) Revision() string {
	if i.commit == "" {
		return devRevision
	}

	return i.commit
}

func (i Info) Announce(next http.Handler) http.Handler {
	if i.commit == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(CommitHeader, i.commit)
		next.ServeHTTP(w, r)
	})
}
