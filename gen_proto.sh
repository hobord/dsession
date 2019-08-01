protoc -I ./session/ -I ../../../ --go_out=plugins=grpc:./session/ ./session/session.proto
