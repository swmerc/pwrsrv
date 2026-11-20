# Power switch API

This server controls the power outlets on a networked power switch. You will
be given its base URL separately; it's written as `$BASE_URL` below (for
example `http://127.0.0.1:8000`, with no trailing slash). Every endpoint lives
under `$BASE_URL/api/outlets`.

There is no authentication, no API key and no special headers. Requests and
responses are small, and everything is plain HTTP.

## Outlets and how to name them

Each outlet has:

- **index**: its number, starting at 1, matching the label on the hardware
- **name**: a free-form name set on the switch (may contain spaces)
- **on**: whether it is currently powered

Wherever an endpoint takes `{id}`, you can give either the outlet's number or
its name:

- A number must be written plainly (`2`, not `02` or `+2`) and be between 1
  and the number of outlets. Numbers are checked before names.
- Otherwise `{id}` is matched exactly (case-sensitive) against outlet names.
- Percent-encode names in the URL: `lava lamp` becomes `lava%20lamp`, `&`
  becomes `%26`, `/` becomes `%2F`.
- An `{id}` that matches nothing returns `400 Bad Request` with the body
  `outlet not found: <id>`.

If you aren't sure what an outlet is called, list them first. Prefer numbers
when you're about to change something, since names can be renamed.

## Endpoints

| Method | Path                          | Request body       | Success response                 |
|--------|-------------------------------|--------------------|----------------------------------|
| GET    | `/api/outlets`                | none               | 200, JSON array of outlets       |
| GET    | `/api/outlets/{id}`           | none               | 200, JSON outlet                 |
| GET    | `/api/outlets/{id}/state`     | none               | 200, `text/plain` `on` or `off`  |
| PUT    | `/api/outlets/{id}/state`     | `on`/`off`         | 200, empty body                  |
| GET    | `/api/outlets/{id}/name`      | none               | 200, `text/plain` name           |
| PUT    | `/api/outlets/{id}/name`      | new name           | 200, empty body                  |

### List all outlets

```
curl -s "$BASE_URL/api/outlets"
```

```json
[
  {"index": 1, "name": "sprinkler", "on": false},
  {"index": 2, "name": "disco ball", "on": true}
]
```

The array is ordered by `index`.

### Get one outlet

```
curl -s "$BASE_URL/api/outlets/2"
curl -s "$BASE_URL/api/outlets/disco%20ball"
```

```json
{"index": 2, "name": "disco ball", "on": true}
```

### Get an outlet's state

```
curl -s "$BASE_URL/api/outlets/sprinkler/state"
```

The body is exactly `on` or `off`, with no trailing newline.

### Turn an outlet on or off

```
curl -s -X PUT "$BASE_URL/api/outlets/1/state" --data "on"
curl -s -X PUT "$BASE_URL/api/outlets/sprinkler/state" --data "off"
```

The body must be `on`, `off`, `true` or `false` (any case, surrounding
whitespace ignored). Anything else returns `400 Bad Request` with body
`unexpected status`. No `Content-Type` is needed. Setting an outlet to the
state it is already in is fine and succeeds.

A 200 means the switch accepted the change. The `on` value reported by the
GET endpoints is the outlet's physical state, so if you read it back
immediately and it hasn't changed yet, wait a second and check again.

### Get an outlet's name

```
curl -s "$BASE_URL/api/outlets/2/name"
```

The body is the name as plain text, with no trailing newline.

### Rename an outlet

```
curl -s -X PUT "$BASE_URL/api/outlets/3/name" --data "lava lamp"
curl -s -X PUT "$BASE_URL/api/outlets/lava%20lamp/name" --data "heater"
```

The body is the new name as plain text, not JSON (don't quote it), with
surrounding whitespace trimmed. An empty name returns `400 Bad Request` with
body `empty name`. Avoid names that are plain numbers: since numbers are
matched first, an outlet named `2` can't be reached by name.

## Errors

Error responses are `text/plain` with a short message and a trailing newline.

| Status | Meaning                                                              |
|--------|----------------------------------------------------------------------|
| 400    | Unknown outlet (`outlet not found: ...`) or bad body (`unexpected status`, `empty name`) |
| 404    | No such path (check for typos; there is nothing outside `/api/outlets`) |
| 405    | Wrong method for the path (e.g. PUT on `/api/outlets/{id}`)           |
| 500    | The server couldn't talk to the switch, or the switch rejected the request; the message has details. Retrying once is reasonable. |

A request can take up to around 10 seconds if the switch is slow, so use a
client timeout of at least 15 seconds.

## Notes for using this safely

- These outlets control real hardware. Only change an outlet's state when you
  have been asked to, and confirm which outlet you mean (by listing them) if
  the request is at all ambiguous.
- Read the state before changing it when it matters, and report back what you
  changed and what the outlet's state was afterwards.
- There is no endpoint to power-cycle an outlet; do it as an `off`, a pause,
  then an `on`.
