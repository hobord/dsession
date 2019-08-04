package session

import (
	"context"
	"reflect"
	"testing"
)

func TestCreateRedisImpl(t *testing.T) {
	s := CreateRedisImpl()
	st := reflect.TypeOf(s).String()
	if st != "*session.GrpcRedisImplServer" {
		t.Errorf("I Got %v", st)
	}
}

func TestCreateSession(t *testing.T) {
	s := GrpcRedisImplServer{}

	// set up test cases
	tests := []struct {
		ttl  int64
		want string
	}{
		{
			ttl: 0,
		},
		{
			ttl: 500,
		},
	}

	for _, tt := range tests {
		req := &CreateSessionMessage{Ttl: tt.ttl}
		resp, err := s.CreateSession(context.Background(), req)
		if err != nil {
			t.Errorf("CreateSession(%v) got unexpected error", tt.ttl)
		}
		if resp.Id == "" {
			t.Errorf("CreateSession(%v), wanted result string", tt.ttl)
		}
	}
}
