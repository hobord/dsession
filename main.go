//protoc -I ./session/ -I ../../../ --go_out=plugins=grpc:./session/ ./session/session.proto
// go get github.com/fullstorydev/grpcurl
// go install github.com/fullstorydev/grpcurl/cmd/grpcurl
/*
// docker run -p 6379:6379 --name redis-redisjson redislabs/rejson:latest
grpcurl.exe -plaintext localhost:50051 list

grpcurl -plaintext -d '{"ttl":10}' localhost:50051 hobord.session.DSessionService/CreateSession
grpcurl -plaintext -d '{"ttl":0}' localhost:50051 hobord.session.DSessionService/CreateSession
grpcurl -plaintext -d '{"id":"8f60aaef-a0bd-4c55-ab49-00c4ed5a4091", "key":"foo", "value": {"numberValue": 15}}' localhost:50051 hobord.session.DSessionService/AddValueToSession
grpcurl -plaintext -d '{"id":"8f60aaef-a0bd-4c55-ab49-00c4ed5a4091"}'  localhost:50051 hobord.session.DSessionService/GetSession

*/

package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/hobord/dsession/session"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":50051"
	}
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	fmt.Println("Server listen: ", port)

	s := grpc.NewServer()
	reflection.Register(s)

	pbImpl := pb.CreateRedisImpl()
	pb.RegisterDSessionServiceServer(s, pbImpl)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
