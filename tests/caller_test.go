package router_test

import (
	"errors"
	"testing"

	"github.com/tinywasm/router"
	"github.com/tinywasm/router/mock"
)

func TestCallerContract(t *testing.T) {
	var _ router.Caller = (*mock.Caller)(nil)
}

func TestMockCaller_Call(t *testing.T) {
	m := &mock.Caller{
		CannedResult: []byte("ok"),
		CannedError:  nil,
	}

	var called bool
	m.Call("test_op", nil, func(res []byte, err error) {
		called = true
		if string(res) != "ok" {
			t.Errorf("expected 'ok', got %q", string(res))
		}
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	if !called {
		t.Error("callback was not called")
	}

	if len(m.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.Calls))
	}
	if m.Calls[0].Op != "test_op" {
		t.Errorf("expected op 'test_op', got %q", m.Calls[0].Op)
	}
}

func TestMockCaller_Dispatch(t *testing.T) {
	m := &mock.Caller{}
	m.Dispatch("fire_and_forget", nil)

	if len(m.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.Calls))
	}
	if m.Calls[0].Op != "fire_and_forget" {
		t.Errorf("expected op 'fire_and_forget', got %q", m.Calls[0].Op)
	}
}

func TestMockCaller_Error(t *testing.T) {
	errTest := errors.New("test error")
	m := &mock.Caller{
		CannedError: errTest,
	}

	var called bool
	m.Call("fail_op", nil, func(res []byte, err error) {
		called = true
		if err != errTest {
			t.Errorf("expected %v, got %v", errTest, err)
		}
	})

	if !called {
		t.Error("callback was not called")
	}
}
