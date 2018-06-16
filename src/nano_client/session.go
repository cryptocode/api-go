package nano_client

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"nano_api"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
)

// Encapsulates error state. The error code is non-zero for valid errors, and
// message and category are usually set for errors trasmitted by the node.
// Implements the Go error interface.
type Error struct {
	Code     int
	Message  string
	Category string
}

// Returns an error string in the format ERRORCODE:CATEGORY:MESSAGE where
// the CATEGORY is optional.
// Example: 4:error_common.invalid_signature
func (e *Error) Error() string {
	if len(e.Category) > 0 {
		return fmt.Sprintf("%d:%s:%s", e.Code, e.Category, e.Message)
	} else {
		return fmt.Sprintf("%d:%s", e.Code, e.Message)
	}
}

// Represents a session with a Nano node
type Session struct {
	connection net.Conn
	connected  bool
	// The last error or nil. This is reset on every call on the Session.
	LastError *Error
	// Read and Write timeout. Default is 30 seconds.
	TimeoutReadWrite int
	// Connection timeout. Default is 10 seconds.
	TimeoutConnection int
}

// Connect to a node. You can set Session#TimeoutConnection before this call, otherwise a default
// of 10 seconds is used.
// connectionString is an URI of the form tcp://host:port or local:///path/to/domainsocketfile
func (s *Session) Connect(connectionString string) *Error {
	s.LastError = nil

	uri, err := url.Parse(connectionString)
	if err != nil {
		s.LastError = &Error{1, "Invalid connection string", "Connection"}
	} else {
		scheme := uri.Scheme
		host := uri.Host
		if scheme == "local" {
			scheme = "unix"
			host = uri.Path
		} else if scheme != "tcp" {
			s.LastError = &Error{1, "Invalid schema: Use tcp or local.", "Connection"}
		}

		// Use a default of 10s if not set by user
		if s.TimeoutConnection == 0 {
			s.TimeoutConnection = 10
		}
		if s.TimeoutReadWrite == 0 {
			s.TimeoutReadWrite = 30
		}
		dialContext := (&net.Dialer{
			KeepAlive: 30 * time.Second,
			Timeout:   time.Duration(s.TimeoutConnection) * time.Second,
		}).DialContext

		con, err := dialContext(context.Background(), scheme, host)
		if err != nil {
			s.LastError = &Error{1, err.Error(), "Connection"}
			s.connected = false
		} else {
			s.connection = con
			s.connected = true
		}
	}

	return s.LastError
}

// Updates the write deadline
func (s *Session) updateWriteDeadline() {
	s.connection.SetWriteDeadline(time.Now().Add(time.Duration(s.TimeoutReadWrite) * time.Second))
}

// Updates the read deadline
func (s *Session) updateReadDeadline() {
	s.connection.SetReadDeadline(time.Now().Add(time.Duration(s.TimeoutReadWrite) * time.Second))
}

// Query the node. The session must be connected.
// Returns a message representing the result or an error.
func (s *Session) Query(query proto.Message, response proto.Message) (proto.Message, *Error) {
	s.LastError = nil
	var result proto.Message = nil
	if !s.connected {
		s.LastError = &Error{1, "Not connected", "Network"}
	} else {
		query_type := strings.ToUpper(strings.Replace(proto.MessageName(query), "nano.api.query_", "", 1))
		log.Println(query_type + ":" + strconv.FormatInt(int64(nano_api.QueryType_value[query_type]), 10))

		query_header := &nano_api.Query{
			Type: nano_api.QueryType(nano_api.QueryType_value[query_type]),
		}

		if query_header.Type == nano_api.QueryType_UNKOWN {
			panic("Invalid query type:" + query_type)
		}

		header_data, err := proto.Marshal(query_header)
		if err != nil {
			s.LastError = &Error{1, err.Error(), "Marshalling"}
		} else {

			// TODO: write reserved uint32

			log.Printf("Header is %d bytes", len(header_data))
			var buf_len [4]byte
			binary.BigEndian.PutUint32(buf_len[:], uint32(len(header_data)))

			s.updateWriteDeadline()
			_, err := s.connection.Write(buf_len[:])
			if err != nil {
				s.LastError = &Error{1, err.Error(), "Network"}
			} else {
				s.updateWriteDeadline()
				_, err = s.connection.Write(header_data[:])

				msg_buffer, err := proto.Marshal(query)
				if err != nil {
					s.LastError = &Error{1, err.Error(), "Marshalling"}
				} else {
					binary.BigEndian.PutUint32(buf_len[:], uint32(len(msg_buffer)))
					s.updateWriteDeadline()
					_, err := s.connection.Write(buf_len[:])
					if err != nil {
						s.LastError = &Error{1, err.Error(), "Network"}
					} else {
						s.updateWriteDeadline()
						_, err := s.connection.Write(msg_buffer[:])
						if err != nil {
							s.LastError = &Error{1, err.Error(), "Network"}
						} else {
							// Get result
							s.updateReadDeadline()
							_, err := io.ReadFull(s.connection, buf_len[:])
							if err != nil {
								s.LastError = &Error{1, err.Error(), "Network"}
							} else {
								buf_response_header := make([]byte, binary.BigEndian.Uint32(buf_len[:]))
								s.updateReadDeadline()
								_, err := io.ReadFull(s.connection, buf_response_header)
								if err != nil {
									s.LastError = &Error{1, err.Error(), "Network"}
								} else {
									resp_header := &nano_api.Response{}
									err := proto.Unmarshal(buf_response_header, resp_header)
									if err != nil {
										s.LastError = &Error{1, err.Error(), "Network"}
									} else {
										s.updateReadDeadline()
										_, err := io.ReadFull(s.connection, buf_len[:])
										if err != nil {
											s.LastError = &Error{1, err.Error(), "Network"}
										} else {
											buf_response := make([]byte, binary.BigEndian.Uint32(buf_len[:]))
											s.updateReadDeadline()
											_, err := io.ReadFull(s.connection, buf_response)
											if err != nil {
												s.LastError = &Error{1, err.Error(), "Network"}
											} else {
												err := proto.Unmarshal(buf_response, response)
												if err != nil {
													s.LastError = &Error{1, err.Error(), "Network"}
												} else {
													log.Println("GOT RESPONSE: " + response.String())
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return result, s.LastError
}

func (s *Session) GetLastError() *Error {
	return s.LastError
}
