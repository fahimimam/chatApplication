# chatd — User Guide

A small terminal chat app. You connect to a shared server (`chatd`)
with a terminal client (`chattui`) and exchange messages with other
users from your command line. No web browser, no electron app — just
a TUI that runs over plain TCP.

This document is for **end users** (the people who type messages), not
for the person who runs the server. If you also need to set up a
server, see `README.md` at the project root.

---

## What you can do

- **Sign up** for an account with a username and password.
- **Log in** to your existing account from any terminal that can reach
  the server.
- **Open a direct conversation** with any other user by their username.
- **Send and receive** text messages in real time across multiple
  conversations.
- **Browse history** — open a conversation and scroll back through
  older messages.
- **List all your conversations** and jump back into any of them.
- **Switch users** — log out and log in as someone else without
  restarting the client.

## What's deliberately not included (yet)

- Group conversations or channels — only direct (1:1) chats.
- Editing or deleting messages you sent.
- File or image attachments.
- Typing indicators / read receipts.
- Push notifications when a new message arrives while you're in
  another conversation (refresh the list with `r`).
- Encrypted transport — the server uses plain TCP. Don't run it on an
  untrusted network.

---

## Getting started

You'll need two things running:

1. A `chatd` server someone has set up for you, reachable at
   `host:port`.
2. The `chattui` binary or `go run ./cmd/chattui`.

### Step 1 — connect

```sh
chattui -addr <server>:<port>
```

For a local server:

```sh
go run ./cmd/chattui -addr 127.0.0.1:7171
```

You should see a screen like:

```
chatd

mode: login
press ctrl+r to switch between login and register

[ username________________ ]
[ ••••••••              ]

tab: switch field   enter: submit   ctrl+r: login/register   ctrl+c: quit
```

### Step 2 — create your account

The first time you use the app you'll need to register. Press
**Ctrl+R** to switch the mode from `login` to `register`. The screen
will now show `mode: register (creating a new account)`.

Type your username and a password, press Tab to move between fields,
then press Enter. Pick a password you can remember — there's no
"forgot password" flow yet.

After registering, you can press Ctrl+R again to switch back to login
mode (your account is now permanent).

### Step 3 — start chatting

Once you're logged in you'll see the conversation list:

```
chats — alice

> direct   bob
  direct   carol

↑/↓: move   enter: open   n: new chat   r: refresh   ctrl+c: quit
```

- **↑ / ↓** (or `k` / `j`) move the selection.
- **Enter** opens the highlighted conversation in the chat view.
- **n** opens the inline "new chat" prompt.
- **r** refreshes the list (useful if someone else just messaged you).

### Step 4 — send your first message

In the chat view, type into the bottom input box and press Enter. Your
message appears immediately at the bottom of the conversation log,
and the server sends it to the other participant.

```
chat — bob
────────────────────────────────────────────────────────
[14:02:11] alice  hello!
[14:02:14] bob    hey, what's up?
────────────────────────────────────────────────────────
type a message and press Enter_

enter: send   esc: back to chats   ctrl+c: quit
```

While you're typing:

- **Enter** sends the message.
- **Esc** returns to the conversation list without sending.
- **Ctrl+C** quits the client (your session on the server is closed
  too).

### Step 5 — start a brand-new conversation

From the conversation list, press **n**. The chat view opens with no
conversation loaded and the input prompt becomes:

```
open: <username>
```

Type the other person's username (e.g. `dave`) and press Enter. The
client will:

1. Create the conversation on the server (or reuse one if you already
   have a direct chat with that user).
2. Switch to the chat view.
3. Load any existing message history.

You can do the same from inside the chat view with the slash command:

```
/open dave
```

---

## Slash commands (inside the chat view)

| Command     | What it does                                              |
|-------------|-----------------------------------------------------------|
| `/open <u>` | Opens (or reuses) a direct conversation with username `u`.|
| `/quit`     | Logs out and exits the client.                            |
| `/exit`     | Same as `/quit`.                                          |

Anything else that starts with `/` is shown as `unknown command: …`.

---

## Tips and gotchas

- **Two people on the same machine can each run their own `chattui`**
  with different logins. The server keeps them separate.
- **Messages you send appear with your username** on the right side
  of the chat log (purple), and incoming messages are on the left
  (green) so you can tell them apart at a glance.
- **The history view shows the most recent 100 messages.** Older ones
  are stored on the server and will be paged in automatically in a
  future version; for now, ask your admin to bump the limit if you
  need more.
- **If you forget your password**, ask the server admin — there's no
  self-serve reset yet.
- **If the server is down**, the client will say so on startup and
  exit. There's no automatic reconnect.
- **If your terminal looks garbled**, make sure your `TERM` is set to
  something colour-capable (e.g. `xterm-256color`, `screen-256color`,
  or `tmux-256color`). The TUI uses 256-colour ANSI sequences.

---

## Quick reference — all keyboard shortcuts

| Screen        | Key             | Action                                  |
|---------------|-----------------|-----------------------------------------|
| Login         | `tab`           | Switch between username and password    |
| Login         | `ctrl+r`        | Toggle between login and register mode  |
| Login         | `enter`         | Submit                                  |
| Login         | `ctrl+c`        | Quit                                    |
| Conversations | `↑` / `↓`       | Move selection                          |
| Conversations | `enter`         | Open the highlighted conversation       |
| Conversations | `n`             | New chat prompt                         |
| Conversations | `r`             | Refresh list                            |
| Conversations | `esc`           | Quit                                    |
| Chat          | (type + enter)  | Send a message                          |
| Chat          | `esc`           | Back to conversation list               |
| Chat          | `ctrl+c`        | Quit                                    |
| Anywhere      | `ctrl+c`        | Quit (always works)                     |

---

## A worked example

Two users, Alice and Bob, want to chat. They share a `chatd` server at
`chat.example.com:7171`.

**Alice's terminal:**

```sh
$ chattui -addr chat.example.com:7171
# mode is "login". Press Ctrl+R to switch to "register".
# Type "alice" / "hunter2", press Enter.
# Press Ctrl+R to switch back to "login". Log in again.
# (From now on she just logs in.)
chats — alice
(empty list — press n to start a new chat)

> (press n, then type "bob" and Enter)
chat — bob
────────────────────────────────────────────────────────
(no messages yet — say hi!)
────────────────────────────────────────────────────────
hi bob! _
```

**Bob's terminal (after Alice's message):**

```sh
$ chattui -addr chat.example.com:7171
# Log in as bob / hispassword.
# He sees "alice" in the conversation list.
# Press Enter to open the chat.
chat — alice
────────────────────────────────────────────────────────
[14:02:11] alice  hi bob!
────────────────────────────────────────────────────────
hello alice! how's it going? _
```

That's the whole loop. Type, press Enter, get replies.
