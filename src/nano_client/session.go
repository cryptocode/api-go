package nano_client

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"nano_api"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

// Error encapsulates the error code, message and category.
// The Code is non-zero to indicate errors. Message and
// Category are usually set for errors transmitted by the node.
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

// A Session with a Nano node.
type Session struct {
	mutex      sync.Mutex
	connection net.Conn
	// True if the session has been connected to the node
	Connected bool
	// Read and Write timeout. Default is 30 seconds.
	TimeoutReadWrite int
	// Connection timeout. Default is 10 seconds.
	TimeoutConnection int
}

// Connect to a node. You can set Session#TimeoutConnection before this call, otherwise a default
// of 15 seconds is used.
// connectionString is an URI of the form tcp://host:port or local:///path/to/domainsocketfile
func (s *Session) Connect(connectionString string) *Error {
	var connError *Error
	uri, err := url.Parse(connectionString)
	if err != nil {
		connError = &Error{1, "Invalid connection string", "Connection"}
	} else {
		scheme := uri.Scheme
		host := uri.Host
		if scheme == "local" {
			scheme = "unix"
			host = uri.Path
		} else if scheme != "tcp" {
			connError = &Error{1, "Invalid schema: Use tcp or local.", "Connection"}
		}

		if s.TimeoutConnection == 0 {
			s.TimeoutConnection = 15
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
			connError = &Error{1, err.Error(), "Connection"}
			s.Connected = false
		} else {
			s.connection = con
			s.Connected = true
		}
	}

	return connError
}

// Close the underlying connection to the node
func (s *Session) Close() *Error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var err *Error
	if s.Connected {
		s.Connected = false
		closeErr := s.connection.Close()
		if closeErr != nil {
			err = &Error{1, closeErr.Error(), "Connection"}
		}
	}
	return err
}

// Updates the write deadline using Session#TimeoutReadWrite
func (s *Session) updateWriteDeadline() {
	s.connection.SetWriteDeadline(time.Now().Add(time.Duration(s.TimeoutReadWrite) * time.Second))
}

// Updates the read deadline using Session#TimeoutReadWrite
func (s *Session) updateReadDeadline() {
	s.connection.SetReadDeadline(time.Now().Add(time.Duration(s.TimeoutReadWrite) * time.Second))
}

// A CallChain allows safe chaining of functions. If an error
// occurs, the remaining functions in the chain are not called.
type CallChain struct {
	err *Error
}

// Calls the callback if there are no errors.
// The callback should set the CallChain#err property on error */
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
// This method is threadsafe.
// The response output argument will contain the result if no error is returned.
func (s *Session) Request(request proto.Message, response proto.Message) *Error {

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Protobuf encoding
	const PROTOCOL_ENCODING = 0
	const PROTOCOL_PREAMBLE_LEAD = 'N'

	var err *Error
	if !s.Connected {
		err = &Error{1, "Not connected", "Network"}
	} else {
		sc := &CallChain{}

		var err error
		var preamble [4]byte
		var bufLen [4]byte
		var msgBuffer []byte
		var bufResponseHeader []byte
		var bufResponse []byte
		var headerData []byte

		requestType := strings.ToUpper(strings.Replace(proto.MessageName(request), "nano.api.req_", "", 1))
		requestHeader := &nano_api.Request{
			Type: nano_api.RequestType(nano_api.RequestType_value[requestType]),
		}

		if requestHeader.Type == nano_api.RequestType_INVALID {
			panic("Invalid request type:" + requestType)
		}

		sc.do(func() {
			preamble = [4]byte{
				PROTOCOL_PREAMBLE_LEAD,
				PROTOCOL_ENCODING,
				byte(nano_api.APIVersion_VERSION_MAJOR),
				byte(nano_api.APIVersion_VERSION_MINOR)}
			s.updateWriteDeadline()
			if _, err = s.connection.Write(preamble[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			if headerData, err = proto.Marshal(requestHeader); err != nil {
				sc.err = &Error{1, err.Error(), "Marshalling"}
			}
		}).do(func() {
			binary.BigEndian.PutUint32(bufLen[:], uint32(len(headerData)))
			s.updateWriteDeadline()
			if _, err = s.connection.Write(bufLen[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			s.updateWriteDeadline()
			if _, err = s.connection.Write(headerData[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			if msgBuffer, err = proto.Marshal(request); err != nil {
				sc.err = &Error{1, err.Error(), "Marshalling"}
			}
		}).do(func() {
			binary.BigEndian.PutUint32(bufLen[:], uint32(len(msgBuffer)))
			s.updateWriteDeadline()
			if _, err = s.connection.Write(bufLen[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			s.updateWriteDeadline()
			if _, err = s.connection.Write(msgBuffer[:]); err != nil {
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
			if _, err = io.ReadFull(s.connection, bufLen[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			bufResponseHeader = make([]byte, binary.BigEndian.Uint32(bufLen[:]))
			s.updateReadDeadline()
			if _, err = io.ReadFull(s.connection, bufResponseHeader); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			respHeader := &nano_api.Response{}
			if err = proto.Unmarshal(bufResponseHeader, respHeader); err != nil {
				sc.err = &Error{1, err.Error(), "Marshalling"}
			}
		}).do(func() {
			s.updateReadDeadline()
			if _, err = io.ReadFull(s.connection, bufLen[:]); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			bufResponse = make([]byte, binary.BigEndian.Uint32(bufLen[:]))
			s.updateReadDeadline()
			if _, err = io.ReadFull(s.connection, bufResponse); err != nil {
				sc.err = &Error{1, err.Error(), "Network"}
			}
		}).do(func() {
			if err = proto.Unmarshal(bufResponse, response); err != nil {
				sc.err = &Error{1, err.Error(), "Marshalling"}
			}
		}).failure(func() {
			err = sc.err
		})
	}
	return err
}
