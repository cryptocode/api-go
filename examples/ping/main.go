package main

import (
	"fmt"
	"log"
	"nano_api"
	"nano_client"
	"os"
	"time"
)

// Connect to a Nano node and submit a pending accounts query
// Program argument is a connection string such as local:///tmp/nano
// for unix domain sockets and tcp://localhost:7077 for tcp sockets.
func main() {
	connectionString := "local:///tmp/nano"
	if len(os.Args) > 1 {
		connectionString = os.Args[1]
	}

	session := &nano_client.Session{}
	session.TimeoutConnection = 2

	if session.Connect(connectionString) != nil {
		log.Println(session.LastError.Error())
	} else {
		defer session.Close()

		log.Printf("Ping loop in progress...")
		start := time.Now()
		for i := 0; i < 10000; i++ {
			ping := &nano_api.ReqPing{
				Id: 1000,
			}
			pong := &nano_api.ResPing{}

			_, err := session.Request(ping, pong)
			if err != nil {
				fmt.Println(err.Error())
			}
		}

		elapsed := time.Since(start)
		log.Printf("Pings latecy: %s", elapsed/10000)
	}
}
