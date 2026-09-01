// chattui is the terminal chat client. It speaks the chatd TCP protocol
// via internal/transport/tcp.Client and renders the result with Bubble
// Tea.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fahimimam/chatApplication/internal/transport/tcp"
	"github.com/fahimimam/chatApplication/internal/tui"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7171", "chatd server address")
	flag.Parse()

	client, err := tcp.Dial(*addr)
	if err != nil {
		log.Fatalf("dial %s: %v", *addr, err)
	}

	model := tui.NewModel()
	model.SetClient(client)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
	}
	_ = client.Close()
}
