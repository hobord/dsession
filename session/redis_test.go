package session

import (
	"reflect"
	"testing"
	// "github.com/gomodule/redigo/redis"
	// "github.com/rafaeljusto/redigomock"
)

// type RedisPool interface {
// 	ActiveCount() int
// 	Close() error
// 	Get() redis.Conn
// 	GetContext(ctx context.Context) (redis.Conn, error)
// 	IdleCount() int
// 	Stats() redis.PoolStats
// }

func TestCreateRedisImpl(t *testing.T) {
	s := CreateRedisImpl()
	st := reflect.TypeOf(s).String()

	if st != "*session.GrpcRedisImplServer" {
		t.Errorf("I Got %v", st)
	}
}

// func TestCreateSession(t *testing.T) {
// 	mockPool := &redis.Pool{
// 		// Other pool configuration not shown in this example.
// 		Dial: func() (redis.Conn, error) {
// 			c := redigomock.NewConn()
// 			// var response map[string]string
// 			c.Command("HGETALL")
// 			return c, nil
// 		},
// 	}
// 	s := GrpcRedisImplServer{
// 		RedisPool: mockPool,
// 	}

// 	// 	// set up test cases
// 	tests := []struct {
// 		ttl  int64
// 		want string
// 	}{
// 		{
// 			ttl: 0,
// 		},
// 		{
// 			ttl: 500,
// 		},
// 	}

// 	for _, tt := range tests {
// 		req := &CreateSessionMessage{Ttl: tt.ttl}
// 		resp, err := s.CreateSession(context.Background(), req)
// 		if err != nil {
// 			t.Errorf("CreateSession got unexpected error: %v", err)
// 		}
// 		if resp.Id == "" {
// 			t.Errorf("CreateSession(%v), wanted result string", tt.ttl)
// 		}
// 	}
// }
