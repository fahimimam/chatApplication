package main

import (
	"bufio"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var log = logrus.New()

const (
	PORT          = 3000
	ServerMessage = "server"
)

var (
	connectionsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tcp_chat_connections",
		Help: "Number of active connections",
	})
	commandsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tcp_chat_commands_total",
			Help: "Total number of commands received",
		},
		[]string{"command"},
	)
)

func init() {
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)
	prometheus.MustRegister(connectionsGauge)
	prometheus.MustRegister(commandsCounter)
}

type CommandID string

const (
	CmdNickname CommandID = "/name"
	CmdRooms    CommandID = "/rooms"
	CmdJoin     CommandID = "/join"
	CmdMsg      CommandID = "/msg"
	CmdQuit     CommandID = "/quit"
	CmdLeave    CommandID = "/leave"
)

const ChatRoomBufferSize = 100

type Command struct {
	ID     CommandID
	Client *Client
	Args   []string
}

type CircularBuffer struct {
	messages []string
	size     int
	start    int
	end      int
	count    int
	mutex    sync.Mutex
}

func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		messages: make([]string, size),
		size:     size,
	}
}

func (cb *CircularBuffer) Add(message string) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.messages[cb.end] = message
	cb.end = (cb.end + 1) % cb.size
	if cb.count == cb.size {
		cb.start = (cb.start + 1) % cb.size
	} else {
		cb.count++
	}
}

func (cb *CircularBuffer) GetAll() []string {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	result := make([]string, cb.count)
	for i := 0; i < cb.count; i++ {
		result[i] = cb.messages[(cb.start+i)%cb.size]
	}
	return result
}

type Room struct {
	Name     string
	Clients  sync.Map // key: net.Conn, value: *Client
	Messages *CircularBuffer
}

func NewRoom(name string, messageBufferSize int) *Room {
	return &Room{
		Name:     name,
		Messages: NewCircularBuffer(messageBufferSize),
	}
}

type Server struct {
	Rooms    sync.Map // key: string, value: *Room
	Commands chan Command
}

func (s *Server) NewClient(conn net.Conn) {
	connectionsGauge.Inc()
	defer connectionsGauge.Dec()

	log.WithFields(logrus.Fields{
		"remote_addr": conn.RemoteAddr().String(),
	}).Info("new client has connected")

	c := &Client{
		Conn:         conn,
		NickName:     "Anonymous",
		Commands:     s.Commands,
		RateLimiter:  time.NewTicker(time.Second),
		InitialTimer: time.NewTimer(time.Millisecond),
	}
	c.ShowMenu()
	c.ReadInput()
}

var commandDescriptions = map[string]string{
	"/join [room name]":  "Join a chat room",
	"/rooms":             "List available chat rooms",
	"/msg [message]":     "Send a message to the current room",
	"/name [name]":       "Change your nickname",
	"/leave [room name]": "Leave a room",
	"/quit":              "Disconnect from the chat server",
}

func formatMenu() string {
	var commands []string
	var descriptions []string
	maxCommandLength := 0

	for cmd, desc := range commandDescriptions {
		commands = append(commands, cmd)
		descriptions = append(descriptions, desc)
		if len(cmd) > maxCommandLength {
			maxCommandLength = len(cmd)
		}
	}

	menu := "Available commands:\n"
	menu += fmt.Sprintf("%-*s | %s\n", maxCommandLength, "Command", "Description")
	menu += fmt.Sprintf("%s-+-%s\n", strings.Repeat("-", maxCommandLength), strings.Repeat("-", 40))

	for i := 0; i < len(commands); i++ {
		menu += fmt.Sprintf("%-*s | %s\n", maxCommandLength, commands[i], descriptions[i])
	}

	return menu
}
func (c *Client) ShowMenu() {
	menu := formatMenu()
	c.Conn.Write([]byte(menu))
}

func (s *Server) Run() {
	for cmd := range s.Commands {
		commandsCounter.WithLabelValues(string(cmd.ID)).Inc()

		log.WithFields(logrus.Fields{
			"command_id": cmd.ID,
			"client":     cmd.Client.Conn.RemoteAddr().String(),
		}).Info("processing command")

		switch cmd.ID {
		case CmdNickname:
			s.NickName(cmd.Client, cmd.Args)
		case CmdRooms:
			s.ListRooms(cmd.Client, cmd.Args)
		case CmdJoin:
			s.Join(cmd.Client, cmd.Args)
		case CmdMsg:
			s.Message(cmd.Client, cmd.Args)
		case CmdQuit:
			s.Quit(cmd.Client, cmd.Args)
		case CmdLeave:
			s.leaveRoom(cmd.Client, cmd.Args)
		}
	}
}
func (s *Server) leaveRoom(client *Client, args []string) {
	if client.Room != nil {
		client.Conn.Write([]byte("You have left " + client.Room.Name + ".\n"))
		room := s.GetRoomOfClient(client)
		room.Clients.Delete(client.Conn)
		client.Room = nil
		s.broadcastMessage(room, client, fmt.Sprintf("%s has left the room.", client.NickName), false)
	} else {
		client.Conn.Write([]byte("You are not in any room\n"))
	}
}
func (s *Server) ListRooms(c *Client, args []string) {
	var roomNames []string
	s.Rooms.Range(func(key, value interface{}) bool {
		roomNames = append(roomNames, key.(string))
		return true
	})

	roomList := strings.Join(roomNames, ", ")
	if len(roomNames) == 0 {
		roomList = "no rooms available"
	}

	_, err := c.Conn.Write([]byte(fmt.Sprintf("Rooms: %s\n", roomList)))
	if err != nil {
		log.WithFields(logrus.Fields{
			"client": c.Conn.RemoteAddr().String(),
			"error":  err.Error(),
		}).Error("failed to write room list to client")
	}
}

func (s *Server) NickName(c *Client, args []string) {
	if len(args) < 2 {
		c.Error(fmt.Errorf("nickname is required. usage: /name NEW_NICKNAME"))
		return
	}

	c.NickName = args[1]
	_, err := c.Conn.Write([]byte(fmt.Sprintf("Nickname changed to: %s\n", c.NickName)))
	if err != nil {
		log.WithFields(logrus.Fields{
			"client": c.Conn.RemoteAddr().String(),
			"error":  err.Error(),
		}).Error("failed to write nickname change to client")
	}
}

func (s *Server) Message(c *Client, args []string) {
	msg := strings.Join(args[1:], " ")
	if c.Room == nil {
		c.Error(fmt.Errorf("must join a room"))
		return
	}

	room := s.GetRoomOfClient(c)
	if room == nil {
		c.Error(fmt.Errorf("error retrieving room"))
		return
	}
	//formattedMsg := fmt.Sprintf("%s > %s: %s", room.Name, c.NickName, msg)
	//formattedMsg := formatMessage(c.NickName, msg)
	//room.Messages.Add(formattedMsg)
	s.broadcastMessage(room, c, msg, false)
}

func (s *Server) GetRoomOfClient(c *Client) *Room {
	value, ok := s.Rooms.Load(c.Room.Name)
	if !ok {
		c.Error(fmt.Errorf("room not found"))
		return nil
	}

	return value.(*Room)
}

func (s *Server) broadcastMessage(room *Room, sender *Client, msg string, isServer bool) {
	timestamp := time.Now().Format("15:04:05")
	room.Messages.Add(fmt.Sprintf("\033[34m[%s] [%s] %s: %s\033[0m", timestamp, room.Name, sender.NickName, msg))
	room.Clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		var formattedMsg string
		if isServer {
			formattedMsg = s.formatMessage(sender.NickName, msg, timestamp, room.Name, false, isServer)
		} else if client == sender {
			formattedMsg = s.formatMessage(sender.NickName, msg, timestamp, room.Name, true, isServer)
		} else {
			formattedMsg = s.formatMessage(sender.NickName, msg, timestamp, room.Name, false, isServer)
		}
		client.Conn.Write([]byte(formattedMsg + "\n"))
		return true
	})
}
func (s *Server) formatMessage(username, message, timestamp, roomName string, isSelf, isServer bool) string {
	if isServer {
		return fmt.Sprintf("\033[45m[%s] %s: %s\033[0m", timestamp, roomName, message) // Magenta for system messages
	}
	if isSelf {
		return fmt.Sprintf("\033[32m[%s] [%s] You: %s\033[0m", timestamp, roomName, message)
	}
	return fmt.Sprintf("\033[34m[%s] [%s] %s: %s\033[0m", timestamp, roomName, username, message) // Blue for others
}

func (s *Server) Join(c *Client, args []string) {
	if len(args) < 2 {
		c.Error(fmt.Errorf("room name is required. usage: /join ROOM"))
		return
	}
	s.leaveRoom(c, args)
	roomName := args[1]
	value, ok := s.Rooms.Load(roomName)
	if !ok {
		room := NewRoom(roomName, ChatRoomBufferSize)
		s.Rooms.Store(roomName, room)
		value = room
	}

	room := value.(*Room)
	room.Clients.Store(c.Conn, c)
	c.Room = room

	for _, msg := range room.Messages.GetAll() {
		c.Conn.Write([]byte(msg + "\n"))
	}

	s.broadcastMessage(room, c, fmt.Sprintf("%s joined the room", c.NickName), true)
}

//	func (s *Server) removeClient(client *Client) {
//		client.Conn.Write([]byte("You have left " + "\n"))
//		s.broadcastMessage(client.Room, fmt.Sprintf("%s has left the room\n", client.NickName))
//	}
func (s *Server) Quit(c *Client, args []string) {
	if c.Room != nil {
		c.Room.Clients.Delete(c.Conn)
		s.broadcastMessage(c.Room, c, fmt.Sprintf("%s left the room", c.NickName), true)
		c.Room = nil
	}
	c.Conn.Close()
}

type Client struct {
	Conn         net.Conn
	NickName     string
	Commands     chan Command
	Room         *Room
	RateLimiter  *time.Ticker
	InitialTimer *time.Timer
}

func (c *Client) ReadInput() {
	for {
		msg, err := bufio.NewReader(c.Conn).ReadString('\n')
		if err != nil {
			log.WithFields(logrus.Fields{
				"remote_addr": c.Conn.RemoteAddr().String(),
				"error":       err.Error(),
			}).Error("failed to read from client")
			return
		}
		msg = strings.Trim(msg, "\r\n")
		args := strings.Split(msg, " ")
		go c.processMessage(args)
	}
}

func (c *Client) processMessage(args []string) {
	switch args[0] {
	case "/name":
		c.Commands <- Command{
			ID:     CmdNickname,
			Client: c,
			Args:   args,
		}
	case "/rooms":
		c.Commands <- Command{
			ID:     CmdRooms,
			Client: c,
			Args:   args,
		}
	case "/msg":
		c.Commands <- Command{
			ID:     CmdMsg,
			Client: c,
			Args:   args,
		}
	case "/join":
		c.Commands <- Command{
			ID:     CmdJoin,
			Client: c,
			Args:   args,
		}
	case "/quit":
		c.Commands <- Command{
			ID:     CmdQuit,
			Client: c,
			Args:   args,
		}
	case "/leave":
		c.Commands <- Command{
			ID:     CmdLeave,
			Client: c,
			Args:   args,
		}
	default:
		c.Error(fmt.Errorf("unknown command: %s", args[0]))
	}
}

func (c *Client) Error(err error) {
	c.Conn.Write([]byte(fmt.Sprintf("Error: %s\n", err.Error())))
}

func main() {
	s := &Server{
		Commands: make(chan Command),
	}
	go s.Run()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", PORT))
	if err != nil {
		log.Fatal("unable to start the server ", err.Error())
	}
	defer listener.Close()
	log.Println("Started server on: ", PORT)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Fatal(http.ListenAndServe(":2112", nil))
	}()

	for {
		conn, listeningErr := listener.Accept()
		if listeningErr != nil {
			log.Println("Unable to accept connection ", listeningErr.Error())
		}
		go s.NewClient(conn)
	}
}
