# About

This is the Go client for the Nano Node API

# Build

Clone the repository and install the protoc plugin for Go:

```
git clone https://gitcom.com/nanoapi/api-go
cd api-go
export $GOPATH=`pwd`
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
```

# Test

Start a node with domain sockets activated.

```
cd examples/simple && go run main.go & cd -
```

You can alternatively pass a connection string to main.go, such as "tcp://localhost:7077"

# Updating the client after Protobuf changes

If the Protobuf message specification has changed, a new Go file can be generated using the following command:

```
scripts/protobuf-go.sh
```

Note that most changes are backwards compatible. Please see the `protobuf` repository and notes about versioning and compatibility.

# IDE notes

If using Visual Studio Code, setting `go.inferGopath` to true is recommended. This will add the current workspace path to GOPATH.

A debugger is available via `go get -u github.com/derekparker/delve/cmd/dlv`