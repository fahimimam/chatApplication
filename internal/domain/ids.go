package domain

// SessionToken is an opaque token that authenticates a connected client.
// Real implementations live in the application layer; the domain only cares
// about the type so service signatures stay readable.
type SessionToken string

func (t SessionToken) String() string { return string(t) }
