package main

import (
	"log"
	"time"

	"github.com/gecosys/gsc-go/client"
)

func main() {
	var aliasName = "_geuser_"
	conn := client.GetClient()

	// Open connection to Goldeneye Hubs System with your alias name
	err := conn.OpenConn(aliasName)
	if err != nil {
		log.Panic(err)
	}

	// Send message to yourself every 3 seconds
	go func(conn client.GEHClient) {
		timer := time.NewTimer(3 * time.Second)
		for {
			select {
			case <-timer.C:
				conn.SendMessage(aliasName, []byte("Goldeneye Technologies"))
				timer.Reset(3 * time.Second)
			}
		}
	}(conn)

	// Listen message from Goldeneye Hubs System
	chanMsg, chanErr := conn.Listen()
	for {
		select {
		case msg := <-chanMsg:
			log.Printf("Sender: %s\n", msg.Sender)
			log.Printf("Data: %s\n", string(msg.Data))
			log.Printf("Timestamp: %d\n\n", msg.Timestamp)
		case err := <-chanErr:
			log.Println(err)
		}
	}
}
