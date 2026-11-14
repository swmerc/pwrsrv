# pwrsrv

A small Go proxy that puts a simpler REST API in front of a Digital Loggers
Pro Switch (DLI LP9). Single `main` package, no subpackages.

## Commands

- `go test ./...` runs everything (tests log through logrus; pipe through
  `grep -v level=info` to quiet them)
- `go vet ./...` and `gofmt -l .` should both be clean
- `make native` builds `./pwrsrv`; `make all` also cross-builds into
  `builds/arm` (GOARM=5), `builds/arm64` and `builds/amd64`. The version comes from `git describe`
  via ldflags, so it needs tags (`v0.0.x`) to be meaningful.
- `make clean` removes `builds/` entirely

## Layout

- `main.go`: flags (`--cfgpath`, `--version`), loads config, starts server
- `config.go`: YAML config with defaults; validates the `proswitch` section
- `server.go`: chi router and handlers for `/api/outlets...`
- `webswitch.go`: `ProSwitch`, the digest-auth client for the switch's own
  REST API (`restapi/relay/outlets/...`)
- `fakeswitch_test.go`: `httptest` fake of the switch API used by the
  client and handler tests

## Things that aren't obvious

- **Two numbering schemes.** The public API numbers outlets from 1 to match
  the hardware labels. The switch's REST API counts from 0. `ProSwitch`
  stays 0-based; the server converts in exactly two places:
  `lookupOutlet` (number in URL -> switch index) and `NewOutlet`
  (switch index -> `Index` in responses). Keep it that way.
- **Outlet ids are numbers or names.** `lookupOutlet` always fetches the
  outlet list from the switch, accepts only plainly written numbers within
  the real outlet count, then falls back to matching names. Unknown ids
  return 400 via `errOutletNotFound`; switch failures return 500.
- **URL params can arrive escaped.** chi returns the raw path segment when
  a request escapes anything beyond the defaults (`%26`, `%22`, `%2F`...),
  so `lookupOutlet` runs the id through `url.PathUnescape`. Do the same for
  any new path parameter.
- **Build the switch client once.** Use `NewProSwitch`; it holds the one
  `http.Client` so connections get reused. Don't create transports per
  request (that leaked connections before).
- **Timeouts are linked.** Each switch request is capped at
  `proSwitchTimeout` (5s), and a handler may make two in a row, so the total
  must stay under the local server's 15s `WriteTimeout`. Handlers pass
  `r.Context()` to the client.
- **PUT bodies to the switch are form-encoded `value=...`.** State sends
  a bare `true`/`false`. `SetOutletName` sends the plain name, not JSON:
  on hardware a quoted name kept its quotes. The switch does percent-decode
  the value (`%22` arrived as `"`), so keep escaping it; spaces go as `%20`
  because it's unverified whether the switch reads `+` as a space.
- **Non-2xx from the switch is an error** in `performOperation`. Otherwise a
  rejected PUT looks like success.
- **401 in tests is special.** The digest transport handles 401 itself, so a
  fake returning 401 never exercises our status check. Use 403 or 500 when
  testing rejections.
- **Config rejects unknown keys** (`KnownFields(true)`), so any new config
  field needs a yaml tag, a default in `getConfig` if it has one, a mention
  in `example_config.yml`, and a test in `config_test.go`.
- **The server has no auth.** `server.address` defaults to `127.0.0.1`,
  which is how it's deployed. Don't change that default.

## Docs

`README.txt` is the user-facing API documentation, with example requests and
responses. Update it whenever endpoint behaviour, status codes or response
formats change.

When the user agrees to a commit, first bring `README.txt` and this file up
to date with the changes being committed, then commit them together. Don't
commit and fix the docs afterwards.
