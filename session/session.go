package session

import (
	"context"
	"errors"
	fmt "fmt"
	"log"

	"encoding/json"

	st "github.com/golang/protobuf/ptypes/struct"
	"github.com/gomodule/redigo/redis"
	"github.com/nitishm/go-rejson"
	uuid "github.com/satori/go.uuid"
)

// GrpcServer is used to implement session.
type GrpcServer struct {
	RedisConnection redis.Conn
	RedisJSON       *rejson.Handler
}

// CreateSession is create a new empty session
func (s *GrpcServer) CreateSession(ctx context.Context, in *CreateSessionMessage) (*SessionResponse, error) {
	log.Printf("Received: %v", in.Ttl)
	RedisConnection := s.RedisConnection
	RedisJSON := s.RedisJSON

	uuid, err := uuid.NewV4()
	var values map[string]*st.Value
	if err != nil {
		fmt.Printf("Something went wrong: %s", err)
		return &SessionResponse{Id: "", Values: values}, err
	}

	type emptySession struct {
		data map[string]string
	}
	res, err := RedisJSON.JSONSet(uuid.String(), ".", emptySession{})
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

// AddValueToSession is add value into the existing session
func (s *GrpcServer) AddValueToSession(ctx context.Context, in *AddValueToSessionMessage) (*SessionResponse, error) {
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

// AddValuesToSession is add multiple values into the session
func (s *GrpcServer) AddValuesToSession(ctx context.Context, in *AddValuesToSessionMessage) (*SessionResponse, error) {
	var values map[string]*st.Value
	for key, val := range in.Values {
		msg := AddValueToSessionMessage{Id: in.Id, Key: key, Value: val}
		_, err := s.AddValueToSession(ctx, &msg)
		if err != nil {
			return &SessionResponse{Id: "", Values: values}, err
		}
	}
	session, err := s.getValuesBySessionID(in.Id)
	if err != nil {
		return &SessionResponse{Id: "", Values: values}, err
	}

	return session, nil
}

// GetSession return the session by id
func (s *GrpcServer) GetSession(ctx context.Context, in *GetSessionMessage) (*SessionResponse, error) {
	session, err := s.getValuesBySessionID(in.Id)
	if err != nil {
		var values map[string]*st.Value
		return &SessionResponse{Id: "", Values: values}, err
	}

	return session, nil
}

// InvalidateSession is delete the session
func (s *GrpcServer) InvalidateSession(ctx context.Context, in *InvalidateSessionMessage) (*SuccessMessage, error) {
	err := s.RedisConnection.Send("DEL", in.Id)
	if err != nil {
		fmt.Printf("Something went wrong: %s", err)
		return &SuccessMessage{Successfull: false}, err
	}
	return &SuccessMessage{Successfull: true}, nil
}

// InvalidateSessionValue is remove one key from the session
func (s *GrpcServer) InvalidateSessionValue(ctx context.Context, in *InvalidateSessionValueMessage) (*SuccessMessage, error) {
	res, err := s.RedisJSON.JSONDel(in.Id, "."+in.Key)
	if err != nil {
		fmt.Printf("Something went wrong: %s", err)
		return &SuccessMessage{Successfull: false}, err
	}
	if res.(string) != "OK" {
		return &SuccessMessage{Successfull: false}, errors.New(res.(string))
	}
	return &SuccessMessage{Successfull: true}, nil
}

// InvalidateSessionValues is remove multiple keys from the session
func (s *GrpcServer) InvalidateSessionValues(ctx context.Context, in *InvalidateSessionValuesMessage) (*SuccessMessage, error) {
	for _, key := range in.Keys {
		msg := InvalidateSessionValueMessage{Id: in.Id, Key: key}
		_, err := s.InvalidateSessionValue(ctx, &msg)
		if err != nil {
			return &SuccessMessage{Successfull: false}, nil
		}

	}
	return &SuccessMessage{Successfull: true}, nil
}

func (s *GrpcServer) getValuesBySessionID(id string) (*SessionResponse, error) {
	RedisJSON := s.RedisJSON
	response := &SessionResponse{Id: id, Values: make(map[string]*st.Value)}
	var jsonValue map[string]interface{}

	input, err := redis.Bytes(RedisJSON.JSONGet(id, "."))
	if err != nil {
		var values map[string]*st.Value
		return &SessionResponse{Id: "", Values: values}, err
	}

	json.Unmarshal(input, &jsonValue)

	for key := range jsonValue {
		fmt.Println(key)
		response.Values[key] = ToValue(jsonValue[key])
	}

	return response, nil
}
