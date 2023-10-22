package chat

type commandID int

const (
	CMD_NICKNAME commandID = iota
	CMD_JOIN
	CMD_ROOMS
	CMD_MSG
	CMD_QUIT
)

type Command struct {
	ID     commandID `json:"id"`
	Client *Client   `json:"client"`
	Args   []string  `json:"args"`
}

// /room
