package main

import (
	"fmt"
	"github.com/fahimimam/chatApplication/chat"
	"log"
	"net"
)

var port int

func main() {
	s := chat.NewServer()
	go s.Run()

	port = 3000
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		log.Fatal("unable to start the server ", err.Error())
	}
	defer listener.Close()
	log.Println("Started server on: ", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Unable to accept connection ", err.Error())
		}

		go s.NewClient(conn)
	}
}
