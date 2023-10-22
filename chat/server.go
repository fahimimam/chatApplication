package chat

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
)

type Server struct {
	Rooms    map[string]*Room `json:"rooms"`
	Commands chan Command     `json:"commands"`
}

func NewServer() *Server {
	return &Server{
		Rooms:    make(map[string]*Room),
		Commands: make(chan Command), // ? /msg -> /join -> /rooms -> /name -> quit
	}
}

func (s *Server) Run() {
	for cmd := range s.Commands {
		switch cmd.ID {
		case CMD_NICKNAME:
			s.NickName(cmd.Client, cmd.Args)
		case CMD_ROOMS:
			s.ListRooms(cmd.Client, cmd.Args)
		case CMD_JOIN:
			s.Join(cmd.Client, cmd.Args)
		case CMD_MSG:
			s.Message(cmd.Client, cmd.Args)
		case CMD_QUIT:
			s.Quit(cmd.Client, cmd.Args)
		}
	}
}

func (s *Server) NewClient(conn net.Conn) {
	log.Printf("new client has connected: %s", conn.RemoteAddr().String())

	c := &Client{
		Conn:     conn,
		NickName: "Anonymous",
		Commands: s.Commands,
	}

	c.ReadInput()
}

func (s *Server) NickName(c *Client, args []string) {
	c.NickName = args[1]
	c.Message(fmt.Sprintf("all right, Server will know you by %s", c.NickName))
}

func (s *Server) Join(c *Client, args []string) {
	roomName := args[1]
	r, ok := s.Rooms[roomName]
	if !ok {
		r = &Room{
			Name:    roomName,
			Members: make(map[net.Addr]*Client),
		}
		s.Rooms[roomName] = r
	}
	r.Members[c.Conn.RemoteAddr()] = c
	s.quitCurrentRoom(c)

	c.Room = r

	r.Broadcast(c, fmt.Sprintf("%s has joined the room", c.NickName))
	c.Message(fmt.Sprintf("Welcome to %s", r.Name))
}

func (s *Server) ListRooms(c *Client, args []string) {
	var rooms []string

	for name := range s.Rooms {
		rooms = append(rooms, name)
	}

	c.Message(fmt.Sprintf("available rooms are %s", strings.Join(rooms, ", ")))
}

func (s *Server) Message(c *Client, args []string) {
	if c.Room == nil {
		c.Error(errors.New("you must join the room first"))
	}
	c.Room.Broadcast(c, c.NickName+" : "+strings.Join(args[1:], " "))
}

func (s *Server) Quit(c *Client, args []string) {
	log.Printf("Client has disconnected: %s", c.Conn.RemoteAddr().String())
	s.quitCurrentRoom(c)
	c.Message("sad to see you go :(")
	c.Conn.Close()
}

func (s *Server) quitCurrentRoom(c *Client) {
	if c.Room != nil {
		delete(c.Room.Members, c.Conn.RemoteAddr())
		c.Room.Broadcast(c, fmt.Sprintf("%s has left the chat", c.NickName))
	}
}
