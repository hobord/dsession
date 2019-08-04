package session

import (
	"context"
	"errors"
	fmt "fmt"
	"log"
	"os"
	"strconv"
	"time"

	"encoding/json"

	proto "github.com/golang/protobuf/proto"
	st "github.com/golang/protobuf/ptypes/struct"
	"github.com/gomodule/redigo/redis"
	"github.com/nitishm/go-rejson"
	uuid "github.com/satori/go.uuid"
)

// GrpcRedisImplServer is used to implement session.
type GrpcRedisImplServer struct {
	RedisConnection redis.Conn
	RedisJSON       *rejson.Handler
}

func newRedisPool(server, password string, maxIdle int, idleTimeout int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     maxIdle,
		IdleTimeout: time.Second * time.Duration(idleTimeout),
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

// CreateRedisImpl is create an instance of redis implementation of session grpc
func CreateRedisImpl() *GrpcRedisImplServer {
	//redis
	// var addr = flag.String("Server", "localhost:6379", "Redis server address")
	// flag.Parse()
	rh := rejson.NewReJSONHandler()
	rdHost := os.Getenv("REDIS_HOST")
	if rdHost == "" {
		rdHost = "localhost"
	}
	rdPort := os.Getenv("REDIS_PORT")
	if rdPort == "" {
		rdPort = "6379"
	}

	rdDbEnv := os.Getenv("REDIS_DB")
	if rdDbEnv == "" {
		rdDbEnv = "0"
	}
	rdDb, err := strconv.Atoi(rdDbEnv)
	if err != nil {
		log.Fatalf("Failed to connect parse redis-server DB (%s)", rdDbEnv)
	}

	password := os.Getenv("REDIS_PASSWORD")

	maxIdleEnv := os.Getenv("REDIS_MAXIDLE")
	if maxIdleEnv == "" {
		maxIdleEnv = "3"
	}
	maxIdle, err := strconv.Atoi(maxIdleEnv)
	if err != nil {
		log.Fatalf("Failed to connect parse redis-server DB (%s)", rdDbEnv)
	}

	maxTimeOutEnv := os.Getenv("REDIS_MAXTIMEOUT")
	if maxTimeOutEnv == "" {
		maxTimeOutEnv = "240"
	}
	maxTimeOut, err := strconv.Atoi(maxTimeOutEnv)
	if err != nil {
		log.Fatalf("Failed to connect parse redis-server DB (%s)", rdDbEnv)
	}

	rediserver := rdHost + ":" + rdPort
	// Redigo Client
	RedisPool := newRedisPool(rediserver, password, maxIdle, maxTimeOut)
	conn := RedisPool.Get()
	log.Printf("Connecting to Redis (%s) DB=%d Success...", rediserver, rdDb)

	rh.SetRedigoClient(conn)
	impl := &GrpcRedisImplServer{
		RedisConnection: conn,
		RedisJSON:       rh,
	}
	return impl
}

// CreateSession is create a new empty session
func (s *GrpcRedisImplServer) CreateSession(ctx context.Context, in *CreateSessionMessage) (*SessionResponse, error) {
	log.Printf("Received: %v", in.Ttl)
	RedisConnection := s.RedisConnection
	RedisJSON := s.RedisJSON

	uuid, err := uuid.NewV4()
	var values map[string]*st.Value
	if err != nil {
		fmt.Printf("Something went wrong: %s", err)
		return &SessionResponse{}, err
	}

	type emptySession struct {
		data map[string]string
	}
	if RedisJSON != nil {
		res, err := RedisJSON.JSONSet(uuid.String(), ".", emptySession{})
		if err != nil {
			return &SessionResponse{}, err
		}
		if res.(string) != "OK" {
			fmt.Printf("Failed to Set into the Redis: %s", res.(string))
			return &SessionResponse{}, err
		}
	}

	if RedisConnection != nil {
		if in.Ttl > 0 {
			ttlstr := fmt.Sprintf("%d", in.Ttl)
			err := RedisConnection.Send("EXPIRE", uuid.String(), ttlstr)
			if err != nil {
				fmt.Printf("Something went wrong: %s", err)
				return &SessionResponse{}, err
			}
		}
		RedisConnection.Flush()
	}

	return &SessionResponse{Id: uuid.String(), Values: values}, nil
}

// AddValueToSession is add value into the existing session
func (s *GrpcRedisImplServer) AddValueToSession(ctx context.Context, in *AddValueToSessionMessage) (*SessionResponse, error) {
	RedisConnection := s.RedisConnection
	RedisJSON := s.RedisJSON

	data := proto.MarshalTextString(in.Value)
	if RedisJSON != nil {
		res, err := RedisJSON.JSONSet(in.Id, "."+in.Key, data)
		if err != nil {
			return &SessionResponse{}, err
		}
		if res.(string) != "OK" {
			fmt.Printf("Failed to Set into the Redis: %s", res.(string))
			return &SessionResponse{}, err
		}
	}

	if RedisConnection != nil {
		RedisConnection.Flush()
	}

	session, err := s.getValuesBySessionID(in.Id)
	if err != nil {
		return session, err
	}

	return session, nil
}

// AddValuesToSession is add multiple values into the session
func (s *GrpcRedisImplServer) AddValuesToSession(ctx context.Context, in *AddValuesToSessionMessage) (*SessionResponse, error) {
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
		return session, err
	}

	return session, nil
}

// GetSession return the session by id
func (s *GrpcRedisImplServer) GetSession(ctx context.Context, in *GetSessionMessage) (*SessionResponse, error) {
	session, err := s.getValuesBySessionID(in.Id)
	if err != nil {
		var values map[string]*st.Value
		return &SessionResponse{Id: "", Values: values}, err
	}

	return session, nil
}

// InvalidateSession is delete the session
func (s *GrpcRedisImplServer) InvalidateSession(ctx context.Context, in *InvalidateSessionMessage) (*SuccessMessage, error) {
	if s.RedisConnection != nil {
		err := s.RedisConnection.Send("DEL", in.Id)
		if err != nil {
			fmt.Printf("Something went wrong: %s", err)
			return &SuccessMessage{Successfull: false}, err
		}
	}
	return &SuccessMessage{Successfull: true}, nil
}

// InvalidateSessionValue is remove one key from the session
func (s *GrpcRedisImplServer) InvalidateSessionValue(ctx context.Context, in *InvalidateSessionValueMessage) (*SuccessMessage, error) {

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
func (s *GrpcRedisImplServer) InvalidateSessionValues(ctx context.Context, in *InvalidateSessionValuesMessage) (*SuccessMessage, error) {
	for _, key := range in.Keys {
		msg := InvalidateSessionValueMessage{Id: in.Id, Key: key}
		_, err := s.InvalidateSessionValue(ctx, &msg)
		if err != nil {
			return &SuccessMessage{Successfull: false}, nil
		}

	}
	return &SuccessMessage{Successfull: true}, nil
}

func (s *GrpcRedisImplServer) getValuesBySessionID(id string) (*SessionResponse, error) {
	RedisJSON := s.RedisJSON
	response := &SessionResponse{Id: id, Values: make(map[string]*st.Value)}
	var jsonValue map[string]interface{}

	input, err := redis.Bytes(RedisJSON.JSONGet(id, "."))
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(input, &jsonValue)
	if err != nil {
		return response, err
	}

	for key, v := range jsonValue {
		json := v.(string)
		val := st.Value{}
		err := proto.UnmarshalText(json, &val)
		if err != nil {
			return response, err
		}
		response.Values[key] = &val
	}

	return response, nil
}
