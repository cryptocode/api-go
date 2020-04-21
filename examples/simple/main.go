package main

import (
	"log"
	"nano_api"
	"nano_client"
	"os"

	"github.com/golang/protobuf/ptypes/wrappers"
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
		pending := &nano_api.ReqAccountPending{
			Count:     10,
			Source:    true,
			Threshold: &wrappers.StringValue{Value: "20000000000"},
			Accounts: []string{
				"xrb_1111111111111111111111111111111111111111111111111111hifc8npp",
				"xrb_3t6k35gi95xu6tergt6p69ck76ogmitsa8mnijtpxm9fkcm736xtoncuohr3"},
		}
		result := &nano_api.ResAccountPending{}

		err := session.Request(pending, result)
		if err != nil {
			log.Fatalln(err.Error())
		} else {
			log.Println("Pending:", pending.String())
			log.Println("Result:", result.String())
		}
	}
}
