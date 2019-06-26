package main

import (
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

	err := session.Connect(connectionString)
	if err != nil {
		log.Fatalln(err.Error())
	} else {
		defer session.Close()

		log.Printf("Ping loop in progress...")
		start := time.Now()
		for i := 0; i < 10000; i++ {
			ping := &nano_api.ReqPing{
				Id: 1000,
			}
			pong := &nano_api.ResPing{}

			err := session.Request(ping, pong)
			if err != nil {
				log.Fatalln(err.Error())
			}
		}

		elapsed := time.Since(start)
		log.Println("Avg. ping roundtrip time including marshalling: %s", elapsed/10000)
	}
}
