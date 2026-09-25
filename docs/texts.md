# Texts a person reads

Every sentence a service says to a person is a method on `texts.Catalog`, and
each language is an unexported type in `internal/texts` that implements it:

```go
rt.env.Texts().TopicStopped()
```

There is no message file, no string key, no global catalog and no generate
step. The compiler is the catalog. A call to a text that does not exist does
not build, a language that lacks a text does not build, and the values a text
mentions are typed parameters. `golang.org/x/text` supplies the plural rules
and nothing else.

The kit ships no `internal/texts`. A package nothing calls is the first thing
"nothing exists that does nothing" deletes, so the first text a project writes
brings the package with it.

## What is a text

A sentence a person reads: a chat message, a button label, a callback answer, a
web page, an email body. What a platform shows on the service's behalf is a
text too, such as a bot's command list and its description. So is a page's
`lang` attribute.

These are not texts, and they stay in English where they are: log lines,
errors returned to a program, metric labels, audit details. Operators and code
read them, and one language keeps them greppable. The panel's copy in
`assets/ui` is operator UI. It stays English and is outside this page.

## The shape

The example is a Telegram bot built from this kit, which seats an agent in a
forum topic. Its one text with a count comes from a second service, so that one
package shows every part. The three files below compiled, passed `go test` and
passed the kit's golangci-lint as `starterkit/internal/texts` when this page was
written.

```
internal/texts/
  texts.go                the package doc, Catalog, the named argument types, Russian()
  russian.go              type russian, one method per text, and the plural helper
  texts_test.go           renders every text into the snapshot
  .snapshots/TestRussian
```

`texts.go`:

```go
// Package texts holds every sentence the service says to a person. Catalog
// lists them, and each language is an unexported type that implements it.
package texts

type AgentName string

type WindowCount int64

type Catalog interface {
	TopicStopped() string
	SeatPrompt() string
	SeatConfirmed(agent AgentName) string
	SeatPromptSpent() string
	ReembedQueued(windows WindowCount) string
}

func Russian() Catalog {
	return russian{}
}
```

`russian.go`, up to the text with a count, which is under
[Plurals](#plurals):

```go
package texts

import (
	"strconv"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
)

type russian struct{}

func (russian) TopicStopped() string {
	return "Этот агент остановлен."
}

func (russian) SeatPrompt() string {
	return "Какого агента посадить в эту тему?"
}

func (russian) SeatConfirmed(agent AgentName) string {
	return "В этой теме теперь отвечает " + string(agent) + "."
}

func (russian) SeatPromptSpent() string {
	return "Агент уже выбран."
}
```

Code that speaks declares `Texts()` on its `Env`, the way code that stores a
time declares `Now()`:

```go
type Env interface {
	Now() time.Time
	Texts() texts.Catalog
	// ...
}
```

`app` answers it:

```go
func (a *app) Texts() texts.Catalog {
	return texts.Russian()
}
```

and a case test's mock answers it with the same function:

```go
env := &EnvMock{TextsFunc: texts.Russian}
```

The call site asks for the sentence by name:

```go
if agent.Stopped {
	rt.countUpdate(ctx, u, outcomeStopped)
	rt.reply(ctx, u.Chat, u.Thread, rt.env.Texts().TopicStopped())

	return
}
```

A function that says several things takes the texts once:

```go
say := rt.env.Texts()

if rt.seatPromptConsumed(u.Chat, u.MessageID) {
	_ = rt.client.AnswerCallbackQuery(ctx, u.CallbackQueryID, say.SeatPromptSpent())
	return
}
```

**Why `Texts()` goes on the `Env`.** The language a deployment speaks belongs
to `app`, as the time does. `app.Now()` is the one clock, and `app.Texts()` is
the one voice. A case gets its texts from its mock like anything else, and when
a second language arrives the choice is made in one method rather than at every
call site.

**Why an interface with one implementation.** Three things, and none of them
waits for a second language:

- `Catalog` is the list of everything the service says, with what each
  sentence mentions, in one place. It is the list a translator works through
  and the list the test checks against, the way `graph.Env` is the list of
  every command ([env-pattern.md](env-pattern.md)).
- The language type is unexported, so no caller can name a language. Every call
  site, every `Env` and every mock already holds a `Catalog`, which is the shape
  they keep when a second language arrives.
- A language that lacks a text does not build, and the error names the text:
  `english does not implement Catalog (missing method ReembedQueued)`. The
  check is `Russian()` returning `russian{}` as a `Catalog`. There is no
  `var _ Catalog = russian{}` beside it, because the constructor is already
  that proof and cannot go stale.

## One method is one whole message

- The call site never joins a text to anything: not a name, not a number, not
  another text. Word order belongs to the language. A sentence assembled at the
  call site fixes one language's order for all of them.
- Every value a sentence mentions is a parameter of a named type declared in
  `texts`: `AgentName`, not `string`, and `WindowCount`, not `int64`. The caller
  converts, `say.SeatConfirmed(texts.AgentName(agent.Name))`. Two `string`
  parameters can be swapped and still compile, and so can two counts.
- A message built from a list takes the list as typed data and lays it out
  itself. Separators, order and the word for "and" belong to the language.

## Names

Name a text `<Surface><Situation>`, in English, with the surface first so that
related texts sort together: `TopicStopped`, `SeatPrompt`, `SeatPromptSpent`.

The name is the translator's note. There is no description field and no
comment, so the name has to be exact. Take `SeatPromptSpent` over
`SeatAlreadyTaken`: the toast answers a second press on a keyboard that has
already been used. It is not about a seat someone else holds, and a translator
reading the second name would write the wrong sentence.

## Plurals

For whole numbers, Russian has three forms:

- `one` for 1, 21, 101;
- `few` for 2–4 and 22–24;
- `many` for 0, 5–20, 25–30 and 111.

CLDR has a fourth form, `other`, but it covers fractions only, and no whole
number reaches it.

Each form is the whole phrase that agrees with the number. That includes the
noun and every verb that follows it, not just an ending. The rest of
`russian.go`:

```go
func (russian) ReembedQueued(windows WindowCount) string {
	n := strconv.FormatInt(int64(windows), 10)

	queued := russianPlural{
		One:  n + " окно встанет в очередь и получит новый вектор",
		Few:  n + " окна встанут в очередь и получат новый вектор",
		Many: n + " окон встанут в очередь и получат новый вектор",
	}

	return "Векторы сброшены: " + queued.Of(int64(windows)) + "."
}

type russianPlural struct {
	One  string
	Few  string
	Many string
}

func (p russianPlural) Of(n int64) string {
	lastTwoDigits := int(n % 100)

	if lastTwoDigits < 0 {
		lastTwoDigits = -lastTwoDigits
	}

	form := plural.Cardinal.MatchPlural(language.Russian, lastTwoDigits, 0, 0, 0, 0)

	if form == plural.One {
		return p.One
	}

	if form == plural.Few {
		return p.Few
	}

	return p.Many
}
```

The first draft of this example ended the forms at `в очередь` and put
`и получат новый вектор` after them. The snapshot then read
`1 окно встанет в очередь и получат новый вектор`: the second verb agrees with
the number too.

The forms are named fields and not three strings in a row, because three
strings in a row can be swapped and still compile. The helper arrives with the
first text that has a count, so a service with no count has no helper.

**Why the last two digits.** `MatchPlural` panics with an index out of range on
a negative number (measured at -1 and -5), and a count that is a difference can
be negative. CLDR picks the form from the absolute value, and Russian's rule
reads only its last digit and its last two digits (`i % 10`, `i % 100`). So the
helper passes the absolute value of the last two digits. That gives the same
form as `MatchPlural` gives the absolute value, for every value from 0 to
200000 and its negative (measured), and it holds for every `int64`. Taking the
absolute value of the whole number would not: `-math.MinInt64` overflows back
to itself, and `MatchPlural` panics on it (measured).

It is two `if`s and not a `switch` because the kit's `exhaustive` does not
count a default. A `switch` with `One`, `Few` and a default was reported for
`Other`, `Zero`, `Two` and `Many` (measured), and three of those are cases
Russian whole numbers never reach.

`golang.org/x/text` is in the kit's `go.mod` as an indirect dependency. The
first plural makes it direct: run `go mod tidy` in that change.

## The test

`TestRussian` calls every method of the catalog and snapshots what it said with
cupaloy, the kit's snapshot tool ([env-pattern.md](env-pattern.md)). It is
already a dependency, so the test adds none:

```go
package texts_test

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/bradleyjkemp/cupaloy/v2"
	"github.com/stretchr/testify/require"

	"starterkit/internal/texts"
)

type rendered struct {
	method string
	text   string
}

func TestRussian(t *testing.T) {
	say := texts.Russian()
	agent := texts.AgentName("hermes")

	all := []rendered{
		{"TopicStopped", say.TopicStopped()},
		{"SeatPrompt", say.SeatPrompt()},
		{"SeatConfirmed", say.SeatConfirmed(agent)},
		{"SeatPromptSpent", say.SeatPromptSpent()},
	}

	for _, n := range []texts.WindowCount{math.MinInt64, -5, -1, 0, 1, 2, 5, 11, 21, 22, 25, 111} {
		all = append(all, rendered{"ReembedQueued", say.ReembedQueued(n)})
	}

	requireEveryMethod(t, all)

	cupaloy.SnapshotT(t, snapshot(all))
}

func requireEveryMethod(t *testing.T, all []rendered) {
	t.Helper()

	methods := make([]string, 0, len(all))

	for _, r := range all {
		methods = append(methods, r.method)
	}

	catalog := reflect.TypeFor[texts.Catalog]()

	for i := range catalog.NumMethod() {
		require.Contains(t, methods, catalog.Method(i).Name)
	}
}

func snapshot(all []rendered) string {
	var out strings.Builder

	for _, r := range all {
		out.WriteString(r.method + "\n\t" + r.text + "\n")
	}

	return out.String()
}
```

**The calls are written out, not found by reflection.** Both were tried, and
this is the simpler one. Reflection finds every method for free but then has to
invent its arguments: a sample per parameter type, a product across a method's
parameters, and a failure for any type it has no sample for. That is some
seventy lines in every service, and the first text that takes a list or a
struct grows it again. Written out, each call is ordinary Go the compiler
checks, a list or a struct argument is a literal, and one reflective loop keeps
the list complete: it reads `Catalog`'s method names, and a text left out of
the list fails the test with `... does not contain "SeatPromptSpent"`.

The label beside each call is typed by hand. The snapshot prints it next to the
text it labels, and that is where a wrong one shows.

What the snapshot gives:

- Every count is tried at `math.MinInt64`, -5, -1, 0, 1, 2, 5, 11, 21, 22, 25
  and 111. Each form a reader can see is then in the diff a reviewer reads.
  That is how the agreement error above was found, and how a panic on a
  negative count would be.
- A new text is one line in the list, and forgetting it fails the test by name.

The snapshot is a second copy of every text. The kit accepts this copy because
a machine writes it and a machine compares it, so it cannot go stale the way a
comment does. `UPDATE_SNAPSHOTS=true go test ./internal/texts/` re-records it.
cupaloy fails the run that writes the snapshot, by design, and the next run
passes. Read the diff before you commit it, because it is what a person will
read.

Behaviour tests compare against the texts, never against a literal:

```go
require.Equal(t, texts.Russian().SeatPromptSpent(), fake.answered[0])
```

A rewording then touches `russian.go` and the snapshot, and no other test.

## The guard

The kit's `.golangci.yaml` turns on `gosmopolitan`, watching for Cyrillic, and
excludes `internal/texts`:

```yaml
linters:
  enable:
    - gosmopolitan
  settings:
    gosmopolitan:
      allow-time-local: true
      watch-for-scripts:
        - Cyrillic
  exclusions:
    rules:
      - path: internal/texts/
        linters:
          - gosmopolitan
```

The linter is part of the golangci-lint the kit already pins, so this is
configuration and not a new tool. Measured: it flagged a Cyrillic literal in a
package outside `internal/texts` and in that package's test, and it passed the
`russian.go` above. A test that compares against a literal is caught along with
the call site that returns one.

`allow-time-local: true` keeps the linter to scripts. Left at its default, it
also reports every use of `time.Local` (measured), and that is a rule about
time zones this page does not make.

It reads string literals only; a rune literal such as `'ё'` is not checked.
Some Cyrillic strings are not texts, such as a user's message in a fixture.
Such a string gets its own exclusion rule, naming the file. It does not get a
`//nolint`, which is a comment. A project whose texts are in another script
adds that script to `watch-for-scripts`.

## Why not a message library

Each option below was measured when this rule was chosen. This is what each
did with the two mistakes that matter:

| | A text missing in Russian | A misnamed argument | What it adds |
|---|---|---|---|
| `x/text/message` with `gotext` | The key (the English format string) reaches the user, and `gotext` exits 0. | The translation is dropped silently. | An extractor, and a generated `init()` that sets a global catalog. |
| go-i18n v2 | English, plus an error that `if err != nil` throws away. With Russian as the bundle default: English and no error. | It renders `<no value>`. | TOML files, a bundle built at boot, the `goi18n` tool. |
| gettext `.po` | The English msgid. | A printf verb that nothing checks. | `.po` files, and a plural model of two English forms plus a C expression. |
| `Catalog` and a type per language | It does not build. | It does not build. | Nothing. |

The file formats exist for translators who do not write Go. Here there are
none. The texts are few, and the developer who writes a feature also writes its
sentences.

## Adding a second language

The change that adds a second language adds all of the following at once.
None of it comes earlier: a language setting that can hold only one value is a
flag nobody selects.

1. `english.go`: `type english struct{}`, with every method of `Catalog`. It
   does not build until it has them all.
2. `For` in `texts.go` picks the catalog for a reader:

   ```go
   func For(code string, fallback language.Tag) Catalog {
   	matcher := language.NewMatcher([]language.Tag{language.Russian, language.English})

   	_, index := language.MatchStrings(matcher, code, fallback.String())

   	return []Catalog{russian{}, english{}}[index]
   }
   ```

   The two lists are in the same order, and that order is the whole contract of
   `index`. The matcher follows CLDR's matching data, so `be` and `kk` land on
   Russian even when the fallback is English. Measure it again in that change.
3. The deployment's default language is one setting, typed `language.Tag` and
   parsed at boot. Boot refuses a tag that has no type, because the matcher
   silently answers an unknown fallback with the first language in its list.
4. `Texts()` on the `Env` keeps its signature and means the deployment default.
   `TextsFor(code string) texts.Catalog` is added for a text that only one
   person reads.
5. `TestEnglish` goes beside `TestRussian`.

Which language each reader gets:

| Who reads it | Language |
|---|---|
| Everyone in a shared place: a group, a channel, a broadcast | The deployment default |
| One person whose request names a language: a Telegram DM or callback answer (`language_code`), a web page (`Accept-Language`) | Theirs, then the default |
| Anyone, through a platform that sends no language | The default |
| Stored, or handed to another service to deliver | The default. See below. |

In a Telegram group, `language_code` is the sender's app language, not the
group's. So a reply that everyone sees uses the default.

## What this does not do

**Reword without a rebuild.** A new sentence needs a build and a deploy. That is
true of every library above, because each one embeds its files. If an operator
must reword texts per deployment, the texts are configuration
([rules.md](rules.md)) and this page is the wrong tool. Then the right tool is
go-i18n with its files mounted at runtime. The same holds when a translator who
does not write Go appears, or when the texts number in the hundreds.

**Re-translate what was stored or passed on.** A sentence written to a table, or
handed to another service, stays in the language it was rendered in. Where the
reader's language matters, send a code and render the sentence where it is
read. Moving rendering from one service to another is a boundary change and is
decided on its own.

**Fractions.** `russianPlural.Of` takes whole numbers. Russian's `other` form
exists only for fractions, so a fractional quantity needs its own helper.

**Check a template at build time.** `html/template` resolves
`{{.Texts.NoSuchText}}` only when the page renders. Every template needs a test
that renders it.

**Catch an English sentence outside `internal/texts`.** The guard knows
scripts, not sentences. Review still reads the call sites.
