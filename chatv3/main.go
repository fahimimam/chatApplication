package main

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rivo/tview"
	"github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
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
	log = logrus.New()
)

type Server struct {
	Rooms       sync.Map // key: string, value: *Room
	Commands    chan Command
	RoomList    *tview.List
	ChatHistory *tview.TextView
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

type Room struct {
	Name     string
	Clients  sync.Map // key: net.Conn, value: *Client
	Messages *CircularBuffer
}

type CircularBuffer struct {
	messages []string
	size     int
	start    int
	end      int
	count    int
	mutex    sync.Mutex
}

type Command struct {
	ID     CommandID
	Client *Client
	Args   []string
}

type Client struct {
	Conn         net.Conn
	NickName     string
	Commands     chan Command
	Room         *Room
	RateLimiter  *time.Ticker
	InitialTimer *time.Timer
}

func main() {
	app := tview.NewApplication()

	// Room List
	roomList := tview.NewList()
	roomList.SetBorder(true).SetTitle("Rooms").SetTitleAlign(tview.AlignCenter)

	// Chat History
	chatHistory := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetScrollable(true).
		ScrollToEnd()
	chatHistory.SetBorder(true).SetTitle("Chat History").SetTitleAlign(tview.AlignCenter)
	chatHistory.SetChangedFunc(func() {
		app.Draw()
	})

	// Input Box
	input := tview.NewInputField().
		SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	input.SetBorder(true).SetTitle("Message").SetTitleAlign(tview.AlignLeft)

	// Layout
	flex := tview.NewFlex().
		AddItem(roomList, 20, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(chatHistory, 0, 4, false).
				AddItem(input, 3, 1, true), 0, 4, true)

	// Key Bindings for Scrolling
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			chatHistory.ScrollToBeginning() // Scroll to the top when Up is pressed
		case tcell.KeyDown:
			chatHistory.ScrollToEnd() // Scroll to the bottom when Down is pressed
		}
		return event
	})

	// Handle Input Submissions
	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := input.GetText()
			fmt.Fprintf(chatHistory, "[yellow]You:[-] %s\n", text)
			input.SetText("")
		}
	})

	server := &Server{
		Commands:    make(chan Command),
		RoomList:    roomList,
		ChatHistory: chatHistory,
	}

	go server.Run()

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
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
		}
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

func (c *Client) Error(err error) {
	c.Conn.Write([]byte(fmt.Sprintf("Error: %s\n", err.Error())))
}
