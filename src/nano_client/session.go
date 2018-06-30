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

// Closes the underlying connection to the node
func (s *Session) Close() *Error {
	s.LastError = nil
	if s.connected {
		err := s.connection.Close()
		if err != nil {
			s.LastError = &Error{1, err.Error(), "Connection"}
		}
	}
	return s.LastError
}

// Updates the write deadline using Session#TimeoutReadWrite
func (s *Session) updateWriteDeadline() {
	s.connection.SetWriteDeadline(time.Now().Add(time.Duration(s.TimeoutReadWrite) * time.Second))
}

// Updates the read deadline using Session#TimeoutReadWrite
func (s *Session) updateReadDeadline() {
	s.connection.SetReadDeadline(time.Now().Add(time.Duration(s.TimeoutReadWrite) * time.Second))
}

// An error-safe call chain
type CallChain struct {
	err *Error
}

// Calls the callback if there are no errors. The callback should set the CallChain#err property on error */
func (sc *CallChain) do(fn func()) *CallChain {
	if sc.err == nil {
		fn()
	}
	return sc
}

// Calls the callback if there's an error
func (sc *CallChain) failure(fn func()) *CallChain {
	if sc.err != nil {
		fn()
	}
	return sc
}

// Send request to the node. The session must be connected.
// Returns a message representing the result or an error.
func (s *Session) Request(request proto.Message, response proto.Message) (proto.Message, *Error) {

	// Protobuf encoding
	const PROTOCOL_ENCODING = 0
	const PROTOCOL_PREAMBLE_LEAD = 'N'

	s.LastError = nil
	var result proto.Message = nil
	if !s.connected {
		s.LastError = &Error{1, "Not connected", "Network"}
	} else {
		sc := &CallChain{}

		var err error
		var preamble [4]byte
		var buf_len [4]byte
		var msg_buffer []byte
		var buf_response_header []byte
		var buf_response []byte
		var header_data []byte

		request_type := strings.ToUpper(strings.Replace(proto.MessageName(request), "nano.api.req_", "", 1))
		log.Println(request_type + ":" + strconv.FormatInt(int64(nano_api.RequestType_value[request_type]), 10))

		request_header := &nano_api.Request{
			Type: nano_api.RequestType(nano_api.RequestType_value[request_type]),
		}

		if request_header.Type == nano_api.RequestType_INVALID {
			panic("Invalid request type:" + request_type)
		}

		sc.do(func() {
			preamble = [4]byte{
				PROTOCOL_PREAMBLE_LEAD,
				PROTOCOL_ENCODING,
				byte(nano_api.APIVersion_VERSION_MAJOR),
				byte(nano_api.APIVersion_VERSION_MINOR)}
			if _, err = s.connection.Write(preamble[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			if header_data, err = proto.Marshal(request_header); err != nil {
				sc.err = &Error{1, err.Error(), "Marshalling"}
			}
		}).do(func() {
			binary.BigEndian.PutUint32(buf_len[:], uint32(len(header_data)))
			s.updateWriteDeadline()
			if _, err = s.connection.Write(buf_len[:]); err != nil {
				// TEST NODE TIMEOUT
				// time.Sleep(3 * time.Second)
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			s.updateWriteDeadline()
			if _, err = s.connection.Write(header_data[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			if msg_buffer, err = proto.Marshal(request); err != nil {
				sc.err = &Error{1, err.Error(), "Marshalling"}
			}
		}).do(func() {
			binary.BigEndian.PutUint32(buf_len[:], uint32(len(msg_buffer)))
			s.updateWriteDeadline()
			if _, err = s.connection.Write(buf_len[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			s.updateWriteDeadline()
			if _, err = s.connection.Write(msg_buffer[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			// Read and verify preamble
			s.updateReadDeadline()
			if _, err = io.ReadFull(s.connection, preamble[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			} else {
				if preamble[0] != PROTOCOL_PREAMBLE_LEAD || preamble[1] != PROTOCOL_ENCODING {
					sc.err = &Error{1, "Invalid preamble", "Network"}
				} else if preamble[2] > 1 {
					sc.err = &Error{1, "Unsupported API version", "API"}
				}
			}
		}).do(func() {
			s.updateReadDeadline()
			if _, err = io.ReadFull(s.connection, buf_len[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			buf_response_header = make([]byte, binary.BigEndian.Uint32(buf_len[:]))
			s.updateReadDeadline()
			if _, err = io.ReadFull(s.connection, buf_response_header); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			resp_header := &nano_api.Response{}
			if err = proto.Unmarshal(buf_response_header, resp_header); err != nil {
				sc.err = &Error{1, err.Error(), "Marshalling"}
			}
		}).do(func() {
			s.updateReadDeadline()
			if _, err = io.ReadFull(s.connection, buf_len[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			buf_response = make([]byte, binary.BigEndian.Uint32(buf_len[:]))
			s.updateReadDeadline()
			if _, err = io.ReadFull(s.connection, buf_response); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			if err = proto.Unmarshal(buf_response, response); err != nil {
				sc.err = &Error{1, err.Error(), "Marshalling"}
			}
		}).failure(func() {
			s.LastError = sc.err
		})
	}
	return result, s.LastError
}

func (s *Session) GetLastError() *Error {
	return s.LastError
}
