# Download latest protobuf definition
mkdir -p protobuf-master/google/protobuf
wget --quiet --no-check-certificate https://raw.githubusercontent.com/cryptocode/raiblocks/c-api/protobuf/core.proto -O protobuf-master/core.proto
wget --quiet --no-check-certificate https://raw.githubusercontent.com/cryptocode/raiblocks/c-api/protobuf/accounts.proto -O protobuf-master/accounts.proto
wget --quiet --no-check-certificate https://raw.githubusercontent.com/cryptocode/raiblocks/c-api/protobuf/util.proto -O protobuf-master/util.proto
wget --quiet --no-check-certificate https://raw.githubusercontent.com/cryptocode/raiblocks/c-api/protobuf/google/protobuf/wrappers.proto -O protobuf-master/google/protobuf/wrappers.proto

# Generate Go files
protoc --proto_path=protobuf-master --go_out=src/nano_api protobuf-master/core.proto
protoc --proto_path=protobuf-master --go_out=src/nano_api protobuf-master/accounts.proto
protoc --proto_path=protobuf-master --go_out=src/nano_api protobuf-master/util.proto