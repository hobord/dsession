package session

import (
	"context"
	fmt "fmt"
	"log"
	"os"
	"strconv"
	"time"

	proto "github.com/golang/protobuf/proto"
	st "github.com/golang/protobuf/ptypes/struct"
	"github.com/gomodule/redigo/redis"
	uuid "github.com/satori/go.uuid"
)

// GrpcRedisImplServer is used to implement session.
type GrpcRedisImplServer struct {
	RedisPool *redis.Pool
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
	redisPool := newRedisPool(rediserver, password, maxIdle, maxTimeOut)
	log.Printf("Connecting to Redis (%s) DB=%d Success...", rediserver, rdDb)

	impl := &GrpcRedisImplServer{
		RedisPool: redisPool,
	}
	return impl
}

// CreateSession is create a new empty session
func (s *GrpcRedisImplServer) CreateSession(ctx context.Context, in *CreateSessionMessage) (*SessionResponse, error) {
	conn := s.RedisPool.Get()
	uuid, err := uuid.NewV4()
	if err != nil {
		return &SessionResponse{}, err
	}
	if in.Ttl > 0 {
		ttlstr := fmt.Sprintf("%d", in.Ttl)
		err = s.addValueToSession(conn, uuid.String(), "__TTL", ttlstr)
		if err != nil {
			return &SessionResponse{}, err
		}
		conn.Send("EXPIRE", uuid.String(), ttlstr)
		err = conn.Flush()
		if err != nil {
			return &SessionResponse{}, err
		}
	}

	var values map[string]*st.Value
	return &SessionResponse{Id: uuid.String(), Values: values}, nil
}

func (s *GrpcRedisImplServer) addValueToSession(conn redis.Conn, id, key, value string) error {
	return conn.Send("HSET", id, key, value)
}

// AddValueToSession is add value into the existing session
func (s *GrpcRedisImplServer) AddValueToSession(ctx context.Context, in *AddValueToSessionMessage) (*SessionResponse, error) {
	conn := s.RedisPool.Get()

	data := proto.MarshalTextString(in.Value)
	err := s.addValueToSession(conn, in.Id, in.Key, data)
	if err != nil {
		return &SessionResponse{}, err
	}
	err = conn.Flush()
	if err != nil {
		return &SessionResponse{}, err
	}

	session, err := s.getValuesBySessionID(conn, in.Id)
	if err != nil {
		return session, err
	}

	return session, nil
}

// AddValuesToSession is add multiple values into the session
func (s *GrpcRedisImplServer) AddValuesToSession(ctx context.Context, in *AddValuesToSessionMessage) (*SessionResponse, error) {
	conn := s.RedisPool.Get()
	var values map[string]*st.Value

	for key, val := range in.Values {
		data := proto.MarshalTextString(val)
		err := s.addValueToSession(conn, in.Id, key, data)
		if err != nil {
			return &SessionResponse{Id: "", Values: values}, err
		}
	}
	err := conn.Flush()
	if err != nil {
		return &SessionResponse{}, err
	}

	session, err := s.getValuesBySessionID(conn, in.Id)
	if err != nil {
		return session, err
	}

	return session, nil
}

// GetSession return the session by id
func (s *GrpcRedisImplServer) GetSession(ctx context.Context, in *GetSessionMessage) (*SessionResponse, error) {
	conn := s.RedisPool.Get()
	session, err := s.getValuesBySessionID(conn, in.Id)
	if err != nil {
		var values map[string]*st.Value
		return &SessionResponse{Id: "", Values: values}, err
	}

	return session, nil
}

// InvalidateSession is delete the session
func (s *GrpcRedisImplServer) InvalidateSession(ctx context.Context, in *InvalidateSessionMessage) (*SuccessMessage, error) {
	conn := s.RedisPool.Get()
	err := conn.Send("DEL", in.Id)
	if err != nil {
		fmt.Printf("Something went wrong: %s", err)
		return &SuccessMessage{Successfull: false}, err
	}
	err = conn.Flush()
	if err != nil {
		return &SuccessMessage{Successfull: false}, err
	}

	return &SuccessMessage{Successfull: true}, nil
}

func (s *GrpcRedisImplServer) invalidateSessionValue(conn redis.Conn, id string, key string) error {
	return conn.Send("HDEL ", id, key)
}

// InvalidateSessionValue is remove one key from the session
func (s *GrpcRedisImplServer) InvalidateSessionValue(ctx context.Context, in *InvalidateSessionValueMessage) (*SuccessMessage, error) {
	conn := s.RedisPool.Get()
	err := s.invalidateSessionValue(conn, in.Id, in.Key)
	if err != nil {
		fmt.Printf("Something went wrong: %s", err)
		return &SuccessMessage{Successfull: false}, err
	}
	err = conn.Flush()
	if err != nil {
		return &SuccessMessage{Successfull: false}, err
	}

	return &SuccessMessage{Successfull: true}, nil
}

// InvalidateSessionValues is remove multiple keys from the session
func (s *GrpcRedisImplServer) InvalidateSessionValues(ctx context.Context, in *InvalidateSessionValuesMessage) (*SuccessMessage, error) {
	conn := s.RedisPool.Get()
	for _, key := range in.Keys {
		err := s.invalidateSessionValue(conn, in.Id, key)
		if err != nil {
			return &SuccessMessage{Successfull: false}, err
		}
	}
	err := conn.Flush()
	if err != nil {
		return &SuccessMessage{Successfull: false}, err
	}

	return &SuccessMessage{Successfull: true}, nil
}

func (s *GrpcRedisImplServer) getValuesBySessionID(conn redis.Conn, id string) (*SessionResponse, error) {
	response := &SessionResponse{Id: id, Values: make(map[string]*st.Value)}

	res, err := conn.Do("HGETALL", id)
	values, err := redis.StringMap(res, err)
	if err != nil {
		return response, err
	}

	for key, hval := range values {
		if key != "__TTL" {
			val := st.Value{}
			err := proto.UnmarshalText(hval, &val)
			if err != nil {
				return response, err
			}

			response.Values[key] = &val
		}
	}

	return response, nil
}
