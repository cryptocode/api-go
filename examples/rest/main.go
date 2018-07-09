package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"nano_client"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

// RestServer is a REST interface to the Node API
type RestServer struct {
	// Session pool
	sessions    []*nano_client.Session
	nextsession int32
	conf        *_Conf
}

type _Conf struct {
	Port     int       `json:"port"`
	Hostname string    `json:"hostname"`
	Node     _ConfNode `json:"node"`
}

type _ConfNode struct {
	Connection string `json:"connection"`
	Poolsize   int    `json:"poolsize"`
}

// Start serving requests
func (server *RestServer) listen() {

	server.conf = &_Conf{
		Hostname: "", Port: 8080,
		Node: _ConfNode{Connection: "local:///tmp/nano", Poolsize: 1},
	}

	if configBytes, err := ioutil.ReadFile("config.json"); err != nil {
		log.Print("No config file found, using defaults")
	} else {
		if err := json.Unmarshal(configBytes, server.conf); err != nil {
			log.Fatal(err)
		}
	}

	server.tryConnectNode()

	http.HandleFunc("/", server.handler)
	http.ListenAndServe(server.conf.Hostname+":"+strconv.Itoa(server.conf.Port), nil)
}

// Try connecting to the Nano node
func (server *RestServer) tryConnectNode() *nano_client.Error {
	var err *nano_client.Error
	server.sessions = make([]*nano_client.Session, 0)
	for i := 0; err == nil && i < server.conf.Node.Poolsize; i++ {
		session := &nano_client.Session{}
		server.sessions = append(server.sessions, session)
		err = session.Connect(server.conf.Node.Connection)
	}
	if err != nil {
		log.Print(err.Message)
	}

	return err
}

// getSession returns the next available session in a round-robin fashion
func (server *RestServer) getSession() *nano_client.Session {
	next := atomic.AddInt32(&server.nextsession, 1)
	next = next % int32(server.conf.Node.Poolsize)
	return server.sessions[next]
}

// Request handler. This automatically translates between JSON and protobuf messages.
func (server *RestServer) handler(resp http.ResponseWriter, req *http.Request) {

	apiIndex := strings.Index(req.URL.Path, "/api/")
	if req.Method == "POST" && apiIndex == 0 {

		// Reconnect if necessary
		if !server.getSession().Connected {
			if err := server.tryConnectNode(); err != nil {
				log.Print(err)
				if json, jsonErr := json.Marshal(err); jsonErr == nil {
					resp.Write(json)
				}
				return
			}
			log.Print("Reconnected successfully to node")
		}

		path := req.URL.Path[5:]
		requestMessage := "nano.api.req_" + path
		responseMessage := "nano.api.res_" + path

		// Get protobuf message types by name
		msgType := proto.MessageType(requestMessage)
		responseMsgType := proto.MessageType(responseMessage)
		if msgType == nil || responseMsgType == nil {
			responseError := &nano_client.Error{
				Code:     1,
				Category: "Marshalling",
				Message:  "Could not find protobuffer message types for " + path,
			}
			if json, jsonErr := json.Marshal(responseError); jsonErr == nil {
				resp.Write(json)
			}
			return
		}

		// Create protobuf message instances
		new := reflect.New(msgType.Elem())
		protomsg := new.Interface().(proto.Message)
		new = reflect.New(responseMsgType.Elem())
		protoresponse := new.Interface().(proto.Message)

		// Unmarshall the JSON request into the protobuf request message
		if err := jsonpb.Unmarshal(req.Body, protomsg); err != nil {
			log.Print(err)
			if json, jsonErr := json.Marshal(err); jsonErr == nil {
				resp.Write(json)
			}
		} else {
			// Request and write result as JSON
			if err := server.getSession().Request(protomsg, protoresponse); err != nil {
				log.Print(err)
				if json, jsonErr := json.Marshal(err); jsonErr == nil {
					resp.Write(json)
				}
			} else {
				m := &jsonpb.Marshaler{EmitDefaults: true}
				m.Marshal(resp, protoresponse)
			}
		}
	} else {
		resp.Write([]byte("Invalid request method. Use POST."))
	}
}

// Start REST server
func main() {
	s := &RestServer{}
	s.listen()
}
