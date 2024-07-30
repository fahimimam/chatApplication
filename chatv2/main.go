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
	log.SetFormatter(&logrus.TextFormatter{})
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
)

const ChatRoomBufferSize = 5

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

	c.ReadInput()
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
		}
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
	if len(args) < 2 {
		c.Error(fmt.Errorf("message is required. usage: /msg ROOM MESSAGE"))
		return
	}

	roomName := args[1]
	msg := strings.Join(args[2:], " ")

	value, ok := s.Rooms.Load(roomName)
	if !ok {
		c.Error(fmt.Errorf("room not found"))
		return
	}

	room := value.(*Room)
	formattedMsg := fmt.Sprintf("%s: %s", c.NickName, msg)
	room.Messages.Add(formattedMsg)
	s.broadcastMessage(room, formattedMsg)
}

func (s *Server) broadcastMessage(room *Room, msg string) {
	room.Clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		client.Conn.Write([]byte(msg + "\n"))
		return true
	})
}

func (s *Server) Join(c *Client, args []string) {
	if len(args) < 2 {
		c.Error(fmt.Errorf("room name is required. usage: /join ROOM"))
		return
	}

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

	s.broadcastMessage(room, fmt.Sprintf("%s joined the room", c.NickName))
}

func (s *Server) Quit(c *Client, args []string) {
	if c.Room != nil {
		c.Room.Clients.Delete(c.Conn)
		s.broadcastMessage(c.Room, fmt.Sprintf("%s left the room", c.NickName))
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
		cmd := strings.TrimSpace(args[0])

		switch cmd {
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
		default:
			c.Error(fmt.Errorf("unknown command: %s", cmd))
		}
	}
}

func (c *Client) processMessage() {

}

func (c *Client) Error(err error) {
	c.Conn.Write([]byte(fmt.Sprintf("Error: %s\n", err.Error())))
}

func main() {
	s := &Server{
		Commands: make(chan Command),
	}
	go s.Run()

	port := 3000
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		log.Fatal("unable to start the server ", err.Error())
	}
	defer listener.Close()
	log.Println("Started server on: ", port)

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
