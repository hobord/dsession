package session

import (
	"context"
	fmt "fmt"
	"log"

	"encoding/json"

	st "github.com/golang/protobuf/ptypes/struct"
	"github.com/gomodule/redigo/redis"
	"github.com/nitishm/go-rejson"
	uuid "github.com/satori/go.uuid"
)

// server is used to implement session.
type Server struct {
	RedisConnection redis.Conn
	RedisJSON       *rejson.Handler
}

type Session struct {
	data map[string]string
}

func (s *Server) CreateSession(ctx context.Context, in *CreateSessionMessage) (*SessionResponse, error) {
	log.Printf("Received: %v", in.Ttl)
	RedisConnection := s.RedisConnection
	RedisJSON := s.RedisJSON

	uuid, err := uuid.NewV4()
	var values map[string]*st.Value
	if err != nil {
		fmt.Printf("Something went wrong: %s", err)
		return &SessionResponse{Id: "", Values: values}, err
	}

	res, err := RedisJSON.JSONSet(uuid.String(), ".", Session{})
	if err != nil {
		return &SessionResponse{Id: "", Values: values}, err
	}
	if res.(string) != "OK" {
		fmt.Println("Failed to Set into the Redis: ")
		return &SessionResponse{Id: "", Values: values}, err
	}

	if in.Ttl > 0 {
		ttlstr := fmt.Sprintf("%d", in.Ttl)
		err := RedisConnection.Send("EXPIRE", uuid.String(), ttlstr)
		if err != nil {
			fmt.Printf("Something went wrong: %s", err)
			return &SessionResponse{Id: "", Values: values}, err
		}
	}
	RedisConnection.Flush()

	return &SessionResponse{Id: uuid.String(), Values: values}, nil
}

func (s *Server) AddValueToSession(ctx context.Context, in *AddValueToSessionMessage) (*SessionResponse, error) {
	RedisConnection := s.RedisConnection
	RedisJSON := s.RedisJSON
	var values map[string]*st.Value

	res, err := RedisJSON.JSONSet(in.Id, "."+in.Key, in.Value)
	if err != nil {
		return &SessionResponse{Id: "", Values: values}, err
	}
	if res.(string) != "OK" {
		fmt.Println("Failed to Set into the Redis: ")
		return &SessionResponse{Id: "", Values: values}, err
	}
	RedisConnection.Flush()

	session, err := s.getValuesBySessionID(in.Id)
	if err != nil {
		return &SessionResponse{Id: "", Values: values}, err
	}

	return session, nil
}
func (s *Server) GetSession(ctx context.Context, in *GetSessionMessage) (*SessionResponse, error) {
	session, err := s.getValuesBySessionID(in.Id)
	if err != nil {
		var values map[string]*st.Value
		return &SessionResponse{Id: "", Values: values}, err
	}

	return session, nil
}

func (s *Server) InvalidateSession(ctx context.Context, in *InvalidateSessionMessage) (*SuccessMessage, error) {
	return &SuccessMessage{Successfull: true}, nil
}
func (s *Server) InvalidateSessionValue(ctx context.Context, in *InvalidateSessionValueMessage) (*SuccessMessage, error) {
	return &SuccessMessage{Successfull: true}, nil
}

func (s *Server) getValuesBySessionID(id string) (*SessionResponse, error) {
	RedisJSON := s.RedisJSON
	response := &SessionResponse{Id: id, Values: make(map[string]*st.Value)}
	var jsonValue map[string]interface{}

	input, err := redis.Bytes(RedisJSON.JSONGet(id, "."))
	if err != nil {
		var values map[string]*st.Value
		return &SessionResponse{Id: "", Values: values}, err
	}

	json.Unmarshal(input, &jsonValue)

	for key, _ := range jsonValue {
		fmt.Println(key)
		response.Values[key] = ToValue(jsonValue[key])
	}

	return response, nil
}
