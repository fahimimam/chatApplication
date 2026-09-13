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
- **Browse the directory** — see everyone registered, not just the ones
  you happen to remember the username of.
- **See who's online** in real time (right-hand sidebar of the TUI).
- **Start a 1:1 chat** with anyone in the directory by selecting them
  and pressing Enter.
- **Send and receive** text messages in real time across multiple
  conversations.
- **Get live notifications** when a message lands in a conversation
  you're not currently viewing — the status bar flashes the sender's
  name and you can press Enter on the conversation to jump in.
- **Switch users** — log out and log in as someone else without
  restarting the client.

## What's deliberately not included (yet)

- Group conversations or channels — only direct (1:1) chats.
- Editing or deleting messages you sent.
- File or image attachments.
- Typing indicators / read receipts.
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

You should see a login screen:

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

After registering, press Ctrl+R to switch back to login mode (your
account is now permanent) and log in normally.

### Step 3 — the main screen

Once you're logged in you'll see a three-pane layout:

```
┌─────────────────┬───────────────────────────────────┬──────────────────┐
│ Conversations   │ Chat — bob                        │ People           │
│                 │                                   │ ● alice  you     │
│ > @ bob         │ [10:14] alice  hey!               │ ● bob            │
│   # family      │ [10:15] bob    sup?               │ ○ carol          │
│                 │                                   │                  │
│                 │ type a message — enter to send    │ online: 2        │
└─────────────────┴───────────────────────────────────┴──────────────────┘
tab: switch pane · ↑/↓: move · enter: open · n: new chat · /: command · ?: help · ctrl+c: quit
```

- **Left — Conversations.** Every chat you've ever had, most recent
  first. The cursor highlights one row.
- **Centre — Chat.** The currently open conversation's message log
  (scrollable) plus an input line at the bottom.
- **Right — People.** Everyone you've ever chatted with or who's in the
  directory, with a green dot for online and a grey dot for offline.
  Press **u** to filter the directory by a substring.

### Step 4 — keyboard cheatsheet

The help bar at the bottom shows the most common shortcuts, but here
is the full set:

| Screen        | Key             | Action                                          |
|---------------|-----------------|-------------------------------------------------|
| Login         | `tab`           | Switch between username and password            |
| Login         | `ctrl+r`        | Toggle between login and register mode          |
| Login         | `enter`         | Submit                                          |
| Login         | `ctrl+c` / `esc`| Quit                                            |
| Main          | `tab`           | Cycle focus: conversations → chat → people      |
| Main          | `shift+tab`     | Cycle focus in reverse                          |
| Conversations | `↑` / `↓`       | Move selection                                  |
| Conversations | `enter`         | Open the highlighted conversation               |
| People        | `↑` / `↓`       | Move selection                                  |
| People        | `enter`         | Open (or create) a chat with the selected user  |
| Anywhere      | `n`             | New chat — type a username, press Enter         |
| Anywhere      | `u`             | Filter people by substring                      |
| Anywhere      | `r`             | Refresh everything (conversations + online)     |
| Anywhere      | `?`             | Show the help overlay                           |
| Anywhere      | `/`             | Enter a slash command (when not typing a msg)   |
| Chat          | (type + enter)  | Send a message                                  |
| Chat          | `esc`           | Back to conversation list (clears current chat) |
| Anywhere      | `ctrl+c`        | Quit (always works)                             |

### Step 5 — send your first message

1. Make sure the **Chat** pane is focused (press `tab` until the
   centre pane has the highlighted border).
2. Type into the bottom input.
3. Press **Enter**.

Your message appears immediately at the bottom of the chat log, and
the server fans it out to anyone else in the conversation. You'll see
their replies arrive as live updates in the same pane.

### Step 6 — start a brand-new conversation

You have three options:

1. From the **People** pane, press `↑`/`↓` to highlight someone, then
   `enter`. If you don't have a chat with them yet, the server will
   create one.
2. From anywhere, press **`n`**, type the username, press Enter.
3. From inside the chat input, type `/open <username>` and press
   Enter.

### Step 7 — live updates

Once you've logged in, the TUI subscribes to server events. The right
pane (People) updates its dots within a second of anyone joining or
leaving. If a message arrives in a conversation you're not currently
viewing, you'll see a notification in the status bar at the bottom
and the conversation will get a marker in the list. Move the cursor
to that conversation and press Enter to view it.

### Step 8 — log out

Type `/logout` in the chat input, or press `ctrl+c`. If you log back
in as someone else, all your conversations will still be there.

---

## Slash commands (inside the chat input)

| Command        | What it does                                              |
|----------------|-----------------------------------------------------------|
| `/open <user>` | Opens (or creates) a direct conversation with `user`.     |
| `/users`       | Refreshes the People pane.                                |
| `/logout`      | Logs out and returns to the login screen.                 |
| `/quit`        | Same as `ctrl+c` — logs out and exits.                    |
| `/exit`        | Alias for `/quit`.                                        |
| `/help` / `?`  | Show the help overlay.                                    |

Anything else that starts with `/` is shown as `unknown command: …`.

---

## Tips and gotchas

- **You can be logged in multiple times.** The server tracks each open
  connection, and your own messages get echoed back to your other
  sessions so the UI stays consistent across windows.
- **Messages you sent** appear with your username on the left side of
  the chat log (purple), and incoming messages use the sender's
  username (green).
- **The history view shows the most recent 200 messages.** Older ones
  are stored on the server and can be paged in by lowering the limit;
  for now, ask your admin to bump the limit if you need more.
- **If you forget your password**, ask the server admin — there's no
  self-serve reset yet.
- **If the server is down**, the client will display an error and the
  status bar will say so. The chat input becomes a no-op; use
  `ctrl+c` to quit and retry once the server is back.
- **If your terminal looks garbled**, make sure your `TERM` is set to
  something colour-capable (e.g. `xterm-256color`, `screen-256color`,
  or `tmux-256color`). The TUI uses 256-colour ANSI sequences.
- **People listed in the directory can be filtered.** Press `u` and
  type any substring of their username — handy when there are many
  users and you only remember a fragment.

---

## A worked example

Two users, Alice and Bob, want to chat. They share a `chatd` server
at `chat.example.com:7171`.

**Alice's terminal:**

```sh
$ chattui -addr chat.example.com:7171
# mode is "login". Press Ctrl+R to switch to "register".
# Type "alice" / "hunter2", press Enter.
# Press Ctrl+R to switch back to "login". Log in again.
# (From now on she just logs in.)

# Right pane shows nobody (no users yet) or just her.
# Press `n`, type "bob", press Enter — creates the conversation.
# Type "hi bob!" in the chat input, press Enter.

chat — bob
[10:02] alice  hi bob!
```

**Bob's terminal (after Alice's message):**

```sh
$ chattui -addr chat.example.com:7171
# Log in as bob / hispassword.
# People pane shows: ● alice, ● bob (you).
# Conversations pane shows: > @ alice.
# Press Enter to open it.
# Type "hello alice! how's it going?" — Enter.
```

That's the whole loop. The TUI stays in sync automatically; you don't
have to refresh manually unless you want to.
