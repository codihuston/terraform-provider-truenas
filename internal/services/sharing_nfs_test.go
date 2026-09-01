package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/deevus/truenas-go/client"
)

// fakeCaller is a minimal truenas.Caller test double.
type fakeCaller struct {
	method string
	params any
	result string
	err    error
}

func (f *fakeCaller) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	f.method = method
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return json.RawMessage(f.result), nil
}

const testShareJSON = `{
	"id": 7,
	"path": "/mnt/tank/media",
	"aliases": [],
	"comment": "Media library",
	"networks": ["10.0.0.0/24"],
	"hosts": ["nas-client"],
	"ro": true,
	"maproot_user": "root",
	"maproot_group": "root",
	"mapall_user": null,
	"mapall_group": null,
	"security": ["SYS"],
	"enabled": true,
	"expose_snapshots": false,
	"locked": false
}`

func testCreateOpts() CreateNFSShareOpts {
	maprootUser := "root"
	return CreateNFSShareOpts{
		Path:        "/mnt/tank/media",
		Comment:     "Media library",
		Networks:    []string{"10.0.0.0/24"},
		Hosts:       []string{"nas-client"},
		ReadOnly:    true,
		MaprootUser: &maprootUser,
		Security:    []string{"SYS"},
		Enabled:     true,
	}
}

func assertShare(t *testing.T, share *NFSShare) {
	t.Helper()

	if share == nil {
		t.Fatal("expected share, got nil")
	}
	if share.ID != 7 {
		t.Errorf("expected ID 7, got %d", share.ID)
	}
	if share.Path != "/mnt/tank/media" {
		t.Errorf("expected path '/mnt/tank/media', got %q", share.Path)
	}
	if share.Comment != "Media library" {
		t.Errorf("expected comment 'Media library', got %q", share.Comment)
	}
	if !share.ReadOnly {
		t.Error("expected ro true")
	}
	if !share.Enabled {
		t.Error("expected enabled true")
	}
	if share.ExposeSnapshots {
		t.Error("expected expose_snapshots false")
	}
	if share.MaprootUser == nil || *share.MaprootUser != "root" {
		t.Errorf("expected maproot_user 'root', got %v", share.MaprootUser)
	}
	if share.MapallUser != nil {
		t.Errorf("expected mapall_user nil, got %v", *share.MapallUser)
	}
	if share.Locked == nil || *share.Locked {
		t.Errorf("expected locked false, got %v", share.Locked)
	}
	if !reflect.DeepEqual(share.Networks, []string{"10.0.0.0/24"}) {
		t.Errorf("unexpected networks: %v", share.Networks)
	}
	if !reflect.DeepEqual(share.Hosts, []string{"nas-client"}) {
		t.Errorf("unexpected hosts: %v", share.Hosts)
	}
	if !reflect.DeepEqual(share.Security, []string{"SYS"}) {
		t.Errorf("unexpected security: %v", share.Security)
	}
	if !reflect.DeepEqual(share.Aliases, []string{}) {
		t.Errorf("expected empty aliases, got %v", share.Aliases)
	}
}

func TestSharingNFSService_Create(t *testing.T) {
	c := &fakeCaller{result: testShareJSON}
	s := NewSharingNFSService(c)

	share, err := s.Create(context.Background(), testCreateOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "sharing.nfs.create" {
		t.Errorf("expected method 'sharing.nfs.create', got %q", c.method)
	}

	params, ok := c.params.(map[string]any)
	if !ok {
		t.Fatalf("expected map params, got %T", c.params)
	}
	if params["path"] != "/mnt/tank/media" {
		t.Errorf("unexpected path param: %v", params["path"])
	}
	if params["ro"] != true {
		t.Errorf("unexpected ro param: %v", params["ro"])
	}
	// aliases is read-only and must never be submitted.
	if _, ok := params["aliases"]; ok {
		t.Error("expected no aliases param")
	}
	if params["mapall_user"] != (*string)(nil) {
		t.Errorf("expected nil mapall_user param, got %v", params["mapall_user"])
	}

	assertShare(t, share)
}

// nil slices must be sent as [] rather than null, which the API rejects.
func TestNFSOptsToParams_NilListsBecomeEmpty(t *testing.T) {
	params := nfsOptsToParams(CreateNFSShareOpts{Path: "/mnt/tank/media"})

	for _, key := range []string{"networks", "hosts", "security"} {
		if !reflect.DeepEqual(params[key], []string{}) {
			t.Errorf("expected empty %s param, got %v", key, params[key])
		}
	}
	if _, ok := params["aliases"]; ok {
		t.Error("expected no aliases param")
	}
}

func TestSharingNFSService_Create_CallError(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.Create(context.Background(), testCreateOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSharingNFSService_Create_BadJSON(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{result: `not json`})

	if _, err := s.Create(context.Background(), testCreateOpts()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSharingNFSService_Get(t *testing.T) {
	c := &fakeCaller{result: testShareJSON}
	s := NewSharingNFSService(c)

	share, err := s.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "sharing.nfs.get_instance" {
		t.Errorf("expected method 'sharing.nfs.get_instance', got %q", c.method)
	}
	if c.params != int64(7) {
		t.Errorf("expected params 7, got %v", c.params)
	}
	assertShare(t, share)
}

func TestSharingNFSService_Get_NotFound(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{err: enoentRPCError()})

	share, err := s.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if share != nil {
		t.Fatalf("expected nil share, got %+v", share)
	}
}

func TestSharingNFSService_Get_OtherError(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.Get(context.Background(), 7); err == nil {
		t.Fatal("expected error")
	}
}

func TestSharingNFSService_Get_BadJSON(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{result: `{`})

	if _, err := s.Get(context.Background(), 7); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSharingNFSService_List(t *testing.T) {
	c := &fakeCaller{result: `[` + testShareJSON + `]`}
	s := NewSharingNFSService(c)

	shares, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "sharing.nfs.query" {
		t.Errorf("expected method 'sharing.nfs.query', got %q", c.method)
	}
	if len(shares) != 1 {
		t.Fatalf("expected 1 share, got %d", len(shares))
	}
	assertShare(t, &shares[0])
}

func TestSharingNFSService_List_CallError(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSharingNFSService_List_BadJSON(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{result: `{}`})

	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSharingNFSService_Update(t *testing.T) {
	c := &fakeCaller{result: testShareJSON}
	s := NewSharingNFSService(c)

	share, err := s.Update(context.Background(), 7, testCreateOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "sharing.nfs.update" {
		t.Errorf("expected method 'sharing.nfs.update', got %q", c.method)
	}

	args, ok := c.params.([]any)
	if !ok || len(args) != 2 {
		t.Fatalf("expected [id, data] params, got %v", c.params)
	}
	if args[0] != int64(7) {
		t.Errorf("expected id 7, got %v", args[0])
	}
	assertShare(t, share)
}

func TestSharingNFSService_Update_CallError(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.Update(context.Background(), 7, testCreateOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSharingNFSService_Update_BadJSON(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{result: `[]`})

	if _, err := s.Update(context.Background(), 7, testCreateOpts()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSharingNFSService_Delete(t *testing.T) {
	c := &fakeCaller{result: `true`}
	s := NewSharingNFSService(c)

	if err := s.Delete(context.Background(), 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "sharing.nfs.delete" {
		t.Errorf("expected method 'sharing.nfs.delete', got %q", c.method)
	}
	if c.params != int64(7) {
		t.Errorf("expected params 7, got %v", c.params)
	}
}

func TestSharingNFSService_Delete_Error(t *testing.T) {
	s := NewSharingNFSService(&fakeCaller{err: errors.New("boom")})

	if err := s.Delete(context.Background(), 7); err == nil {
		t.Fatal("expected error")
	}
}

// enoentRPCError reproduces the error the live API returns for a missing
// instance: sharing.nfs.get_instance(99999).
func enoentRPCError() *client.JSONRPCError {
	return &client.JSONRPCError{
		Code:    client.ErrCodeTrueNASCall,
		Message: "[ENOENT] None: SharingNFS 99999 does not exist",
		Data: &client.JSONRPCData{
			Reason: "[ENOENT] None: SharingNFS 99999 does not exist",
			Error:  2,
		},
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"enoent rpc error", enoentRPCError(), true},
		{"enoent rpc errno only", &client.JSONRPCError{
			Code:    client.ErrCodeTrueNASCall,
			Message: "SharingNFS 99999 does not exist",
			Data:    &client.JSONRPCData{Error: 2},
		}, true},
		{"wrapped enoent rpc error", fmt.Errorf("get instance: %w", enoentRPCError()), true},
		{"enoent marker", errors.New("[ENOENT] missing"), true},
		{"enoent rpc message only", &client.JSONRPCError{
			Code:    client.ErrCodeTrueNASCall,
			Message: "[ENOENT] None: User 42 does not exist",
		}, true},
		{"rpc message without enoent", &client.JSONRPCError{
			Code:    client.ErrCodeTrueNASCall,
			Message: "User 42 does not exist",
		}, false},
		{"method does not exist", &client.JSONRPCError{Code: -32601, Message: "Method does not exist"}, false},
		{"plain does not exist", errors.New("Entry does not exist"), false},
		{"not found", errors.New("share not found"), false},
		{"no such instance", errors.New("no such instance"), false},
		{"unrelated", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotFoundError(tt.err); got != tt.want {
				t.Errorf("isNotFoundError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestMockSharingNFSService_Defaults(t *testing.T) {
	var m SharingNFSServiceAPI = &MockSharingNFSService{}
	ctx := context.Background()

	if share, err := m.Create(ctx, CreateNFSShareOpts{}); share != nil || err != nil {
		t.Error("expected nil, nil from default Create")
	}
	if share, err := m.Get(ctx, 1); share != nil || err != nil {
		t.Error("expected nil, nil from default Get")
	}
	if shares, err := m.List(ctx); shares != nil || err != nil {
		t.Error("expected nil, nil from default List")
	}
	if share, err := m.Update(ctx, 1, UpdateNFSShareOpts{}); share != nil || err != nil {
		t.Error("expected nil, nil from default Update")
	}
	if err := m.Delete(ctx, 1); err != nil {
		t.Error("expected nil from default Delete")
	}
}

func TestMockSharingNFSService_Overrides(t *testing.T) {
	want := &NFSShare{ID: 3}
	m := &MockSharingNFSService{
		CreateFunc: func(ctx context.Context, opts CreateNFSShareOpts) (*NFSShare, error) { return want, nil },
		GetFunc:    func(ctx context.Context, id int64) (*NFSShare, error) { return want, nil },
		ListFunc:   func(ctx context.Context) ([]NFSShare, error) { return []NFSShare{*want}, nil },
		UpdateFunc: func(ctx context.Context, id int64, opts UpdateNFSShareOpts) (*NFSShare, error) { return want, nil },
		DeleteFunc: func(ctx context.Context, id int64) error { return errors.New("boom") },
	}
	ctx := context.Background()

	if got, _ := m.Create(ctx, CreateNFSShareOpts{}); got != want {
		t.Error("CreateFunc not used")
	}
	if got, _ := m.Get(ctx, 1); got != want {
		t.Error("GetFunc not used")
	}
	if got, _ := m.List(ctx); len(got) != 1 {
		t.Error("ListFunc not used")
	}
	if got, _ := m.Update(ctx, 1, UpdateNFSShareOpts{}); got != want {
		t.Error("UpdateFunc not used")
	}
	if err := m.Delete(ctx, 1); err == nil {
		t.Error("DeleteFunc not used")
	}
}
