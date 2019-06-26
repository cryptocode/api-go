# About

This is the Go client for the Nano Node API

# Build

Clone the repository and install the protoc plugin for Go:

```bash
git clone https://github.com/nanoapi/api-go
cd api-go
export GOPATH=`pwd`
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
go install nano_api
```

# Updating the client after Protobuf changes

If the Protobuf message specification has changed, a new Go source files can be generated using the following command:

```bash
ci/protobuf-gen.sh
```

# Test

Start a node with domain sockets activated and run:

```bash
export GOPATH=`pwd`
cd examples/simple && go run main.go & cd -
```

You can alternatively pass a connection string to main.go, such as "tcp://localhost:7077"

# IDE notes

If using Visual Studio Code, setting `go.inferGopath` to true is recommended. This will add the current workspace path to GOPATH.

A debugger is available via `go get -u github.com/derekparker/delve/cmd/dlv`
