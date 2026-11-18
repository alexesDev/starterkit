# Configuration

Every setting is a flag, and every flag can also be set from a variable whose
name is derived from it: `--metrics-interval` is `STARTERKIT_METRICS_INTERVAL`.
`internal/confread` does the reading, and the package that uses a setting is
the package that declares it.

## Where a setting lives

A service package owns its `Config` type, its defaults, the names of its
settings and the checks on them:

```go
func (c *Config) Register(r *confread.Reader) {
	*c = DefaultConfig()

	r.String(&c.Addr, "metrics-addr", "listener for the Prometheus endpoint")
	r.Duration(&c.UpdateInterval, "metrics-interval", time.Second, "how often gauges are refreshed")
}

func (c *Config) Check(r *confread.Reader) {
	r.Require("metrics-addr", c.Addr != "", "is empty")
}
```

`internal/appconfig` only assembles them. It registers each service, parses
once, runs each check, and owns the handful of settings that belong to the
server itself (`addr`, `public-url`, `base-path`, `database-url`, …) plus the
shutdown pair:

```go
cfg.Gate.Register(r)
cfg.Metrics.Register(r)
// ...
r.Parse(args)

cfg.Gate.Check(r)
cfg.Metrics.Check(r)
// ...
return cfg, r.Err()
```

So adding a setting is one line in the package that reads it, and the env name,
the flag, the `--help` line and the error that names it all follow from that
line. `confread` imports nothing from the project, so any package can register
on it without a cycle. A check that relates two services stays in
`appconfig.check`.

| Package | Settings |
|---|---|
| `appconfig` | `addr`, `public-url`, `base-path`, `database-url`, `repo-url`, `sign-out-url`, `bootstrap-admin`, `dev-mode`, `shutdown-delay`, `shutdown-timeout` |
| `gateauth` | `oidc-issuer`, `oidc-client-id` |
| `audit` | `audit-retention`, `audit-purge-spec` |
| `notifier` | `notifier-from` |
| `clientip` | `trusted-proxies` |
| `metrics` | `metrics-addr`, `metrics-interval` |
| `logger` | `log-level` |

`go run ./cmd/server --help` prints the full list, each line with its variable.

## The order, and what wins

1. The default, from the service's `DefaultConfig()`.
2. The environment, `.env` included: `godotenv` loads it into the process
   environment first. An empty variable is not applied.
3. The command line. Flags are parsed last, so a one-off override never needs
   the environment edited.

The environment is applied through each flag's own `Set`, not turned into the
flag's default. That is why `--help` prints the kit's defaults and never what
the environment holds.

## Every problem, at once, and a value never in it

Parsing does not stop at the first bad value. Every problem is collected and
the process refuses to start with all of them in one error:

```text
fatal: STARTERKIT_SHUTDOWN_DELAY: must be a duration with a unit, such as 30s, of at least 0s
STARTERKIT_TRUSTED_PROXIES: entry 2 is not a CIDR such as 10.0.0.0/8
STARTERKIT_METRICS_ADR names no setting
STARTERKIT_OIDC_CLIENT_ID (--oidc-client-id): is empty
```

- **A malformed value is an error, never the default.** `STARTERKIT_SHUTDOWN_DELAY=10`
  without a unit used to be a silent zero.
- **A `STARTERKIT_` variable that names no setting is an error.** A typo in a
  name is otherwise indistinguishable from a setting left at its default. The
  price: when a setting is deleted from the code, the variable has to leave the
  deployment in the same change, or the next start refuses.
- **A problem names the variable or the flag, never the value.** The value may
  be a secret. `confread`'s own parsers return fixed messages; a `Func` you
  write must do the same — return what was expected, not what arrived.

## The one exception: `DATABASE_URL`

dbmate reads `DATABASE_URL`, unprefixed, and the server must read the same
variable so the two cannot point at different files. `r.Key("database-url",
"DATABASE_URL")` replaces the derived name; `STARTERKIT_DATABASE_URL` is then an
unknown variable like any other.

## Tests

`appconfig.Parse(args, environ)` takes the environment as a slice, so a test
passes exactly the variables it means and never touches the process's own:

```go
_, err := appconfig.Parse(nil, []string{"STARTERKIT_SHUTDOWN_DELAY=10"})
require.ErrorContains(t, err, "STARTERKIT_SHUTDOWN_DELAY: must be a duration")
```

`internal/confread/confread_test.go` pins the mechanism itself: the derived
names, flags over environment, collection, the unknown variable, `--help`
without the environment, and a value that never reaches an error.
