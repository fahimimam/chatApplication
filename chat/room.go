package chat

import "net"

type Room struct {
	Name    string               `json:"name"`
	Members map[net.Addr]*Client `json:"members"`
}

func (r *Room) Broadcast(sender *Client, msg string) {
	for addr, m := range r.Members {
		if addr != sender.Conn.RemoteAddr() {
			m.Message(msg)
		}
	}
}
