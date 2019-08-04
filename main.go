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
	"flag"
	"fmt"
	"log"
	"net"

	pb "github.com/hobord/dsession/session"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/gomodule/redigo/redis"
	"github.com/nitishm/go-rejson"
)

const (
	port = ":50051"
)

func main() {

	//redis
	var addr = flag.String("Server", "localhost:6379", "Redis server address")
	rh := rejson.NewReJSONHandler()
	flag.Parse()
	// Redigo Client
	conn, err := redis.Dial("tcp", *addr)
	if err != nil {
		log.Fatalf("Failed to connect to redis-server @ %s", *addr)
	}
	defer func() {
		err = conn.Close()
		if err != nil {
			log.Fatalf("Failed to communicate to redis-server @ %v", err)
		}
	}()
	rh.SetRedigoClient(conn)

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	reflection.Register(s)
	pb.RegisterDSessionServiceServer(s, &pb.GrpcServer{
		RedisConnection: conn,
		RedisJSON:       rh,
	})
	fmt.Println("Server listen: ", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
