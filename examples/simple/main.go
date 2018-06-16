package main

import (
	"fmt"
	"log"
	"nano_api"
	"nano_client"
	"os"

	"github.com/golang/protobuf/ptypes/wrappers"
)

// Connect to a Nano node and submit a pending accounts query
// Program argument is a connection string such as local:///tmp/nano
// for unix domain sockets and tcp://localhost:7077 for tcp sockets.
//
// By default, we connect via domain sockets. Override by passing the
// connection string as a program argument.
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
		pending := &nano_api.QueryAccountPending{
			Count:     10,
			Threshold: &wrappers.StringValue{Value: "20000000000"},
			Accounts: []string{
				"xrb_1111111111111111111111111111111111111111111111111111hifc8npp",
				"xrb_3t6k35gi95xu6tergt6p69ck76ogmitsa8mnijtpxm9fkcm736xtoncuohr3"},
		}
		result := &nano_api.ResAccountPending{}

		_, err := session.Query(pending, result)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println(result.String())
		}
	}
	session.Close()
	fmt.Println("Done")
}
