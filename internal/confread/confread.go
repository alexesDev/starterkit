// Package confread reads settings from flags and prefixed environment
// variables into the Config types the service packages declare.
//
// Each service registers its own settings on a Reader and checks them; the
// Reader derives every environment name from the flag name, applies the
// environment through the flag's own Set, then lets the command line win, and
// reports every problem at once. A value never appears in a problem, and
// --help prints defaults rather than the environment. See docs/config.md.
package confread

import (
	"errors"
	"flag"
	"io"
	"strconv"
	"strings"
	"time"
)

type Reader struct {
	flags    *flag.FlagSet
	prefix   string
	environ  []string
	keys     map[string]string
	settings []*setting
	problems []error
}

func New(name, prefix string, environ []string) *Reader {
	return &Reader{
		flags:   flag.NewFlagSet(name, flag.ContinueOnError),
		prefix:  prefix,
		environ: environ,
		keys:    map[string]string{},
	}
}

func (r *Reader) SetOutput(w io.Writer) {
	r.flags.SetOutput(w)
}

func (r *Reader) String(p *string, name, usage string) {
	set := func(raw string) error {
		*p = raw

		return nil
	}

	r.define(name, usage, *p, set, false)
}

func (r *Reader) Bool(p *bool, name, usage string) {
	set := func(raw string) error {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return errors.New("must be true or false")
		}

		*p = value

		return nil
	}

	r.define(name, usage, strconv.FormatBool(*p), set, true)
}

func (r *Reader) Duration(p *time.Duration, name string, least time.Duration, usage string) {
	set := func(raw string) error {
		value, err := time.ParseDuration(raw)
		if err != nil || value < least {
			return errors.New("must be a duration with a unit, such as 30s, of at least " + least.String())
		}

		*p = value

		return nil
	}

	r.define(name, usage, p.String(), set, false)
}

func (r *Reader) Int(p *int, name string, least int, usage string) {
	set := func(raw string) error {
		value, err := strconv.Atoi(raw)
		if err != nil || value < least {
			return errors.New("must be a whole number of at least " + strconv.Itoa(least))
		}

		*p = value

		return nil
	}

	r.define(name, usage, strconv.Itoa(*p), set, false)
}

func (r *Reader) Func(name, byDefault, usage string, set func(raw string) error) {
	r.define(name, usage, byDefault, set, false)
}

func (r *Reader) Key(name, key string) {
	r.keys[name] = key
}

func (r *Reader) Require(name string, ok bool, problem string) {
	if ok {
		return
	}

	r.problems = append(r.problems, errors.New(r.label(name)+": "+problem))
}

func (r *Reader) Parse(args []string) {
	byKey := map[string]*setting{}

	for _, s := range r.settings {
		key := r.key(s.name)
		byKey[key] = s

		f := r.flags.Lookup(s.name)
		f.Usage = "[" + key + "] " + f.Usage
	}

	for _, line := range r.environ {
		key, raw, _ := strings.Cut(line, "=")

		s, known := byKey[key]
		if !known && strings.HasPrefix(key, r.prefix) {
			r.problems = append(r.problems, errors.New(key+" names no setting"))
		}

		if !known || raw == "" {
			continue
		}

		err := s.set(raw)
		if err != nil {
			r.problems = append(r.problems, errors.New(key+": "+err.Error()))
		}
	}

	err := r.flags.Parse(args)
	if err != nil {
		r.problems = append(r.problems, err)
	}
}

func (r *Reader) Err() error {
	return errors.Join(r.problems...)
}

func (r *Reader) define(name, usage, byDefault string, set func(raw string) error, isBool bool) {
	s := &setting{reader: r, name: name, byDefault: byDefault, set: set, isBool: isBool}
	r.settings = append(r.settings, s)
	r.flags.Var(s, name, usage)
}

func (r *Reader) key(name string) string {
	key, renamed := r.keys[name]
	if renamed {
		return key
	}

	return r.prefix + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

func (r *Reader) label(name string) string {
	return r.key(name) + " (--" + name + ")"
}

type setting struct {
	reader    *Reader
	name      string
	byDefault string
	set       func(raw string) error
	isBool    bool
}

func (s *setting) String() string {
	if s == nil {
		return ""
	}

	return s.byDefault
}

func (s *setting) Set(raw string) error {
	err := s.set(raw)
	if err != nil {
		s.reader.problems = append(s.reader.problems, errors.New("--"+s.name+": "+err.Error()))
	}

	return nil
}

func (s *setting) IsBoolFlag() bool {
	return s.isBool
}
