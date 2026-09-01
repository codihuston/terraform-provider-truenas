package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

const testServiceJSON = `{
	"id": 9,
	"service": "nfs",
	"enable": true,
	"state": "RUNNING",
	"pids": [1234]
}`

func TestSystemServices_Get(t *testing.T) {
	caller := &fakeCaller{result: "[" + testServiceJSON + "]"}

	svc, err := NewSystemServices(caller).Get(context.Background(), "nfs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.method != "service.query" {
		t.Errorf("expected method 'service.query', got %q", caller.method)
	}
	want := []any{[]any{[]any{"service", "=", "nfs"}}}
	if !reflect.DeepEqual(caller.params, want) {
		t.Errorf("expected params %v, got %v", want, caller.params)
	}

	if svc == nil {
		t.Fatal("expected service, got nil")
	}
	if svc.ID != 9 {
		t.Errorf("expected ID 9, got %d", svc.ID)
	}
	if svc.Name != "nfs" {
		t.Errorf("expected name 'nfs', got %q", svc.Name)
	}
	if !svc.Enable {
		t.Error("expected enable true")
	}
	if svc.State != ServiceStateRunning {
		t.Errorf("expected state %q, got %q", ServiceStateRunning, svc.State)
	}
	if !svc.Running() {
		t.Error("expected Running true")
	}
}

// An unknown service name is reported as an empty result set rather than an
// error, so Get must translate that to nil rather than a zero-valued service.
func TestSystemServices_Get_NotFound(t *testing.T) {
	caller := &fakeCaller{result: "[]"}

	svc, err := NewSystemServices(caller).Get(context.Background(), "bogus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != nil {
		t.Fatalf("expected nil service, got %+v", svc)
	}
}

func TestSystemServices_Get_CallError(t *testing.T) {
	caller := &fakeCaller{err: errors.New("boom")}

	if _, err := NewSystemServices(caller).Get(context.Background(), "nfs"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSystemServices_Get_BadJSON(t *testing.T) {
	caller := &fakeCaller{result: "not json"}

	if _, err := NewSystemServices(caller).Get(context.Background(), "nfs"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSystemServices_List(t *testing.T) {
	caller := &fakeCaller{result: `[` + testServiceJSON + `,
		{"id": 4, "service": "ssh", "enable": false, "state": "STOPPED"}]`}

	svcs, err := NewSystemServices(caller).List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.method != "service.query" {
		t.Errorf("expected method 'service.query', got %q", caller.method)
	}
	if caller.params != nil {
		t.Errorf("expected nil params, got %v", caller.params)
	}

	if len(svcs) != 2 {
		t.Fatalf("expected 2 services, got %d", len(svcs))
	}
	if svcs[1].Name != "ssh" {
		t.Errorf("expected name 'ssh', got %q", svcs[1].Name)
	}
	if svcs[1].Running() {
		t.Error("expected Running false for a STOPPED service")
	}
}

func TestSystemServices_List_CallError(t *testing.T) {
	caller := &fakeCaller{err: errors.New("boom")}

	if _, err := NewSystemServices(caller).List(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSystemServices_List_BadJSON(t *testing.T) {
	caller := &fakeCaller{result: "not json"}

	if _, err := NewSystemServices(caller).List(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSystemServices_SetEnable(t *testing.T) {
	caller := &fakeCaller{result: "9"}

	if err := NewSystemServices(caller).SetEnable(context.Background(), "nfs", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.method != "service.update" {
		t.Errorf("expected method 'service.update', got %q", caller.method)
	}
	want := []any{"nfs", map[string]any{"enable": true}}
	if !reflect.DeepEqual(caller.params, want) {
		t.Errorf("expected params %v, got %v", want, caller.params)
	}
}

func TestSystemServices_SetEnable_Error(t *testing.T) {
	caller := &fakeCaller{err: errors.New("boom")}

	if err := NewSystemServices(caller).SetEnable(context.Background(), "nfs", false); err == nil {
		t.Fatal("expected error")
	}
}

func TestSystemServices_Start(t *testing.T) {
	caller := &fakeCaller{result: "true"}

	if err := NewSystemServices(caller).Start(context.Background(), "nfs"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.method != "service.start" {
		t.Errorf("expected method 'service.start', got %q", caller.method)
	}
	// silent=false is what turns a failed start into a real error instead of a
	// bare `false` return.
	want := []any{"nfs", map[string]any{"silent": false}}
	if !reflect.DeepEqual(caller.params, want) {
		t.Errorf("expected params %v, got %v", want, caller.params)
	}
}

func TestSystemServices_Start_Error(t *testing.T) {
	caller := &fakeCaller{err: errors.New("boom")}

	if err := NewSystemServices(caller).Start(context.Background(), "nfs"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSystemServices_Stop(t *testing.T) {
	caller := &fakeCaller{result: "true"}

	if err := NewSystemServices(caller).Stop(context.Background(), "nfs"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.method != "service.stop" {
		t.Errorf("expected method 'service.stop', got %q", caller.method)
	}
	want := []any{"nfs", map[string]any{"silent": false}}
	if !reflect.DeepEqual(caller.params, want) {
		t.Errorf("expected params %v, got %v", want, caller.params)
	}
}

func TestSystemServices_Stop_Error(t *testing.T) {
	caller := &fakeCaller{err: errors.New("boom")}

	if err := NewSystemServices(caller).Stop(context.Background(), "nfs"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMockSystemServices_Defaults(t *testing.T) {
	var mock MockSystemServices
	ctx := context.Background()

	if svc, err := mock.Get(ctx, "nfs"); svc != nil || err != nil {
		t.Errorf("expected nil, nil from Get, got %v, %v", svc, err)
	}
	if svcs, err := mock.List(ctx); svcs != nil || err != nil {
		t.Errorf("expected nil, nil from List, got %v, %v", svcs, err)
	}
	if err := mock.SetEnable(ctx, "nfs", true); err != nil {
		t.Errorf("expected nil from SetEnable, got %v", err)
	}
	if err := mock.Start(ctx, "nfs"); err != nil {
		t.Errorf("expected nil from Start, got %v", err)
	}
	if err := mock.Stop(ctx, "nfs"); err != nil {
		t.Errorf("expected nil from Stop, got %v", err)
	}
}

func TestMockSystemServices_Overrides(t *testing.T) {
	sentinel := errors.New("sentinel")
	ctx := context.Background()

	mock := MockSystemServices{
		GetFunc:       func(context.Context, string) (*SystemService, error) { return &SystemService{Name: "nfs"}, nil },
		ListFunc:      func(context.Context) ([]SystemService, error) { return []SystemService{{Name: "ssh"}}, nil },
		SetEnableFunc: func(context.Context, string, bool) error { return sentinel },
		StartFunc:     func(context.Context, string) error { return sentinel },
		StopFunc:      func(context.Context, string) error { return sentinel },
	}

	svc, err := mock.Get(ctx, "nfs")
	if err != nil || svc == nil || svc.Name != "nfs" {
		t.Errorf("unexpected Get result: %v, %v", svc, err)
	}
	svcs, err := mock.List(ctx)
	if err != nil || len(svcs) != 1 || svcs[0].Name != "ssh" {
		t.Errorf("unexpected List result: %v, %v", svcs, err)
	}
	if err := mock.SetEnable(ctx, "nfs", true); !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel from SetEnable, got %v", err)
	}
	if err := mock.Start(ctx, "nfs"); !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel from Start, got %v", err)
	}
	if err := mock.Stop(ctx, "nfs"); !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel from Stop, got %v", err)
	}
}
