package chat

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

type Client struct {
	Conn     net.Conn       `json:"conn"`
	NickName string         `json:"nickName"`
	Room     *Room          `json:"Room"`
	Commands chan<- Command `json:"commands"`
}

func (c *Client) ReadInput() {
	for {
		msg, err := bufio.NewReader(c.Conn).ReadString('\n')
		if err != nil {
			return
		}
		msg = strings.Trim(msg, "\r\n")
		args := strings.Split(msg, " ")
		cmd := strings.TrimSpace(args[0])

		switch cmd {
		case "/name":
			c.Commands <- Command{
				ID:     CMD_NICKNAME,
				Client: c,
				Args:   args,
			}
		case "/rooms":
			c.Commands <- Command{
				ID:     CMD_ROOMS,
				Client: c,
				Args:   args,
			}
		case "/msg":
			c.Commands <- Command{
				ID:     CMD_MSG,
				Client: c,
				Args:   args,
			}
		case "/join":
			c.Commands <- Command{
				ID:     CMD_JOIN,
				Client: c,
				Args:   args,
			}
		case "/quit":
			c.Commands <- Command{
				ID:     CMD_QUIT,
				Client: c,
				Args:   args,
			}
		default:
			c.Error(fmt.Errorf("Unknown command: %s", cmd))
		}
	}
}

func (c *Client) Error(err error) {
	c.Conn.Write([]byte("Error: " + err.Error() + "\n"))
}

func (c *Client) Message(msg string) {
	c.Conn.Write([]byte("> " + msg + "\n"))
}
