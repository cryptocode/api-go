# Download latest protobuf definition
rm master.tar.gz
wget --no-check-certificate https://github.com/nanoapi/protobuf/archive/master.tar.gz
tar xpvf master.tar.gz

# Generate Java files
protoc --proto_path=protobuf-master --go_out=src/nano_api protobuf-master/core.proto

# Cleanup
rm master.tar.gz
