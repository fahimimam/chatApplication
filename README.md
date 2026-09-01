# chatd

A small chat server and terminal client written in Go.

```
+---------+   TCP   +-------+   SQL   +---------+
| chattui | <-----> | chatd | <-----> | SQLite  |
+---------+         +-------+         +---------+
```

The transport is a simple line-oriented TCP protocol so any client can
join — today there is one terminal client (`chattui`) and the protocol
is designed to be replaced with WebSockets for a future web client
without touching the application logic.

## Layout

```
cmd/
  chatd/        server entrypoint
  chattui/      terminal client entrypoint
internal/
  domain/       pure types and rules (User, Conversation, Message, …)
  application/  use-cases that orchestrate domain + repositories
  repository/   interfaces
    sqlite/     the only concrete implementation
  transport/    wire formats — TCP today, WebSocket tomorrow
    tcp/        server + client + protocol parsing
  tui/          Bubble Tea views
  infra/db/     SQLite open + migrations
  config/       env-driven runtime configuration
  logger/       tiny structured logger
```

The dependency direction is strictly inward: `transport` and `tui`
import `application`; `application` imports `domain` and `repository`;
nothing in the lower layers knows about the wire format.

## Running

### Server

```sh
CHATD_ADDR=127.0.0.1:7171 \
CHATD_DB=./chat.db \
go run ./cmd/chatd
```

Environment variables (with defaults):

| Variable             | Default        | Meaning                               |
|----------------------|----------------|---------------------------------------|
| `CHATD_ADDR`         | `:3000`        | listen address                        |
| `CHATD_DB`           | `chat.db`      | SQLite path (use `:memory:` for tests)|
| `CHATD_LOG_LEVEL`    | `info`         | `debug`, `info`, `warn`, `error`      |
| `CHATD_LOG_FILE`     | (stderr)       | optional log file path                |
| `CHATD_METRICS_ADDR` | (disabled)     | optional `/metrics` listen address    |

### Terminal client

```sh
go run ./cmd/chattui -addr 127.0.0.1:7171
```

Controls:

| Screen          | Keys                                    |
|-----------------|-----------------------------------------|
| Login / register| `tab` switch fields, `ctrl+r` toggle mode, `enter` submit, `ctrl+c` quit |
| Conversation    | `↑/↓` move, `enter` open, `n` new chat, `r` refresh, `esc` quit |
| Chat            | type and `enter` to send, `esc` back, `ctrl+c` quit. Slash commands: `/open <user>`, `/quit` |

## Wire protocol

Every line is one command. Responses are prefixed with `OK` or `ERR`;
sessions end with `BYE`. Commands may be tagged with a correlation id
so the same client connection can multiplex many requests:

```
id 3 send <conv> hello from alice
```

becomes

```
id 3 OK sent id=<uuid> ts=<ms>
```

Commands the server understands:

| Verb      | Args                                  | Notes                        |
|-----------|---------------------------------------|------------------------------|
| `register`| `<user> <pass> [display]`             | creates account              |
| `login`   | `<user> <pass>`                       | binds session to connection  |
| `logout`  | —                                     |                              |
| `ping`    | —                                     | `OK pong`                    |
| `whoami`  | —                                     | `OK user id=… username=…`    |
| `open`    | `<user>`                              | `OK conv id=<uuid> kind=…`   |
| `list`    | —                                     | `OK conv …` × N, `OK (N)`    |
| `send`    | `<conv> <message…>`                   | `OK sent id=<uuid> ts=…`     |
| `history` | `<conv> [before_ms] [limit]`          | `OK msg …` × N, `OK end (N)` |
| `quit`    | —                                     | `BYE bye`                    |

## Tests

```sh
go test ./...
go test ./... -race
```

The repository uses `:memory:` SQLite with a single connection per
process. Modernc SQLite honours `cache=private` for `:memory:` DSNs
which keeps each test goroutine on a private database while preserving
foreign keys and a 5 s busy timeout.

## Design notes

* All authorisation is enforced in the application layer. Repositories
  are dumb stores — there is no per-user SQL filtering.
* `MessageRepository.ListByConversation` orders by `(created_at, id)`
  descending and supports cursor-based pagination on `created_at`. The
  cursor is strictly older-than so paging is stable.
* Sessions are 24 h tokens stored in the `sessions` table; expiry is
  checked by `AuthService.Resolve`, not the repository, so test code
  controls the clock.
* The TCP transport is goroutine-safe — one goroutine per connection —
  and the client multiplexes requests with monotonically increasing
  correlation ids, so the TUI can fire requests concurrently.
