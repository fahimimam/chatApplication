// Package tcp implements the line-oriented chat wire protocol that the
// chatd server speaks and the chattui client connects with.
//
// The protocol is intentionally simple — one command per line, tab-free
// whitespace separated, server responds with one or more lines prefixed
// by "OK ", "ERR ", or "MSG ". The transport only marshals bytes; all
// business rules live in the application layer.
package tcp

// Server -> Client response line prefixes.
const (
	PrefixOK   = "OK "
	PrefixERR  = "ERR "
	PrefixMSG  = "MSG "
	PrefixEVT  = "EVT " // server-pushed event (e.g. incoming message)
	PrefixBYE  = "BYE " // server-initiated close
)

// Client -> Server commands. Each verb corresponds to a method on the
// application services; arguments are space-separated.
const (
	CmdRegister  = "register"   // register <user> <password> [display_name]
	CmdLogin     = "login"      // login <user> <password>
	CmdLogout    = "logout"
	CmdList      = "list"       // list (no args; returns conversations)
	CmdOpen      = "open"       // open <username> (idempotent direct chat)
	CmdHistory   = "history"    // history <conv_id> [before_ms] [limit]
	CmdSend      = "send"       // send <conv_id> <message...>
	CmdWhoami    = "whoami"     // returns current user (auth probe)
	CmdWho       = "who"        // returns online users
	CmdUsers     = "users"      // returns known users (search <substring>)
	CmdSubscribe = "subscribe"  // subscribe to realtime events for the user
	CmdUnsub     = "unsubscribe"
	CmdPing      = "ping"       // keepalive
	CmdQuit      = "quit"
)