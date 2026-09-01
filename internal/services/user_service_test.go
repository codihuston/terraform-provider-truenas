package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deevus/truenas-go/client"
)

// userResponseJSON is a user.get_instance payload from TrueNAS SCALE 25.10.
const userResponseJSON = `{
	"id": 71,
	"uid": 3000,
	"username": "deploy",
	"full_name": "Deployment Account",
	"email": null,
	"home": "/mnt/tank/home/deploy",
	"shell": "/usr/bin/bash",
	"group": {"id": 110, "bsdgrp_gid": 3000, "bsdgrp_group": "deploy"},
	"groups": [91],
	"smb": false,
	"locked": false,
	"password_disabled": true,
	"ssh_password_enabled": false,
	"sshpubkey": "ssh-ed25519 AAAA deploy@example",
	"sudo_commands": [],
	"sudo_commands_nopasswd": ["/usr/bin/systemctl restart app"],
	"builtin": false
}`

// recordingCaller captures the calls a service makes and replays canned
// responses in order. Services that read a record back after writing it make
// more than one call, so responses is indexed by call.
type recordingCaller struct {
	calls     []recordedCall
	responses []string
	response  string
	errs      []error
	err       error
}

type recordedCall struct {
	method string
	params any
}

func (c *recordingCaller) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	index := len(c.calls)
	c.calls = append(c.calls, recordedCall{method: method, params: params})

	if index < len(c.errs) && c.errs[index] != nil {
		return nil, c.errs[index]
	}
	if c.err != nil {
		return nil, c.err
	}
	if index < len(c.responses) {
		return json.RawMessage(c.responses[index]), nil
	}
	return json.RawMessage(c.response), nil
}

// call returns the nth recorded call.
func (c *recordingCaller) call(t *testing.T, index int) recordedCall {
	t.Helper()

	if index >= len(c.calls) {
		t.Fatalf("expected at least %d calls, got %d", index+1, len(c.calls))
	}
	return c.calls[index]
}

// callParams returns the request body of the nth recorded call.
func callParams(t *testing.T, caller *recordingCaller, index int) map[string]any {
	t.Helper()

	params, ok := caller.call(t, index).params.(map[string]any)
	if !ok {
		t.Fatalf("expected map parameters, got %T", caller.call(t, index).params)
	}
	return params
}

// callArgs returns the positional arguments of the nth recorded call.
func callArgs(t *testing.T, caller *recordingCaller, index int) []any {
	t.Helper()

	args, ok := caller.call(t, index).params.([]any)
	if !ok {
		t.Fatalf("expected positional parameters, got %T", caller.call(t, index).params)
	}
	return args
}

func TestUserService_Create(t *testing.T) {
	caller := &recordingCaller{response: userResponseJSON}
	uid := int64(3000)
	group := int64(110)
	password := "correct horse battery staple"

	user, err := NewUserService(caller).Create(context.Background(), CreateUserOpts{
		Username:         "deploy",
		FullName:         "Deployment Account",
		UID:              &uid,
		Group:            &group,
		Home:             "/mnt/tank/home",
		HomeCreate:       true,
		HomeMode:         "700",
		Shell:            "/usr/bin/bash",
		PasswordDisabled: true,
		Password:         &password,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.call(t, 0).method != "user.create" {
		t.Errorf("expected method 'user.create', got %q", caller.call(t, 0).method)
	}

	params := callParams(t, caller, 0)
	for name, want := range map[string]any{
		"username":          "deploy",
		"uid":               uid,
		"group":             group,
		"home":              "/mnt/tank/home",
		"home_create":       true,
		"password":          password,
		"password_disabled": true,
	} {
		if params[name] != want {
			t.Errorf("expected %s %v, got %v", name, want, params[name])
		}
	}

	// Empty collections travel as arrays so the API does not reject a null.
	if groups, ok := params["groups"].([]int64); !ok || len(groups) != 0 {
		t.Errorf("expected an empty groups array, got %v", params["groups"])
	}

	if user.ID != 71 || user.UID != 3000 {
		t.Errorf("expected user 71/3000, got %d/%d", user.ID, user.UID)
	}
	if user.Group != 110 {
		t.Errorf("expected primary group entry 110, got %d", user.Group)
	}
	if user.GroupGID != 3000 {
		t.Errorf("expected primary group gid 3000, got %d", user.GroupGID)
	}
	if user.Email != "" {
		t.Errorf("expected a null email to map to the empty string, got %q", user.Email)
	}
	if user.SSHPublicKey != "ssh-ed25519 AAAA deploy@example" {
		t.Errorf("unexpected ssh public key %q", user.SSHPublicKey)
	}
}

func TestUserService_Create_OmitsUnsetOptionals(t *testing.T) {
	caller := &recordingCaller{response: userResponseJSON}

	if _, err := NewUserService(caller).Create(context.Background(), CreateUserOpts{
		Username: "deploy",
		FullName: "Deployment Account",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := callParams(t, caller, 0)
	for _, name := range []string{"uid", "group", "password"} {
		if _, ok := params[name]; ok {
			t.Errorf("expected %s to be omitted, got %v", name, params[name])
		}
	}
}

func TestUserService_Create_Error(t *testing.T) {
	caller := &recordingCaller{err: errors.New("[EINVAL] user_create.password: Password is required")}

	if _, err := NewUserService(caller).Create(context.Background(), CreateUserOpts{}); err == nil {
		t.Fatal("expected the API error to be returned")
	}
}

func TestUserService_Create_MalformedResponse(t *testing.T) {
	caller := &recordingCaller{response: `not json`}

	if _, err := NewUserService(caller).Create(context.Background(), CreateUserOpts{}); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestUserService_Get(t *testing.T) {
	caller := &recordingCaller{response: userResponseJSON}

	user, err := NewUserService(caller).Get(context.Background(), 71)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.call(t, 0).method != "user.get_instance" {
		t.Errorf("expected method 'user.get_instance', got %q", caller.call(t, 0).method)
	}
	if caller.call(t, 0).params != int64(71) {
		t.Errorf("expected id 71, got %v", caller.call(t, 0).params)
	}
	if user.Username != "deploy" {
		t.Errorf("expected username 'deploy', got %q", user.Username)
	}
}

func TestUserService_Get_NotFound(t *testing.T) {
	caller := &recordingCaller{err: errors.New("[ENOENT] None: User 99999 does not exist")}

	user, err := NewUserService(caller).Get(context.Background(), 99999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != nil {
		t.Errorf("expected a missing user to map to nil, got %+v", user)
	}
}

func TestUserService_Get_Error(t *testing.T) {
	caller := &recordingCaller{err: errors.New("connection refused")}

	if _, err := NewUserService(caller).Get(context.Background(), 71); err == nil {
		t.Fatal("expected the API error to be returned")
	}
}

func TestUserService_Get_MalformedResponse(t *testing.T) {
	caller := &recordingCaller{response: `not json`}

	if _, err := NewUserService(caller).Get(context.Background(), 71); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestUserService_Update(t *testing.T) {
	caller := &recordingCaller{response: userResponseJSON}

	if _, err := NewUserService(caller).Update(context.Background(), 71, UpdateUserOpts{
		Username: "deploy",
		FullName: "Deploy Bot",
		Home:     "/mnt/tank/home/deploy",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.call(t, 0).method != "user.update" {
		t.Errorf("expected method 'user.update', got %q", caller.call(t, 0).method)
	}

	args := callArgs(t, caller, 0)
	if len(args) != 2 || args[0] != int64(71) {
		t.Fatalf("expected [71, params], got %v", args)
	}

	params, ok := args[1].(map[string]any)
	if !ok {
		t.Fatalf("expected map parameters, got %T", args[1])
	}
	if params["full_name"] != "Deploy Bot" {
		t.Errorf("expected full_name 'Deploy Bot', got %v", params["full_name"])
	}
	// uid and group_create are rejected by the API on update.
	for _, name := range []string{"uid", "group_create", "home_create", "password"} {
		if _, ok := params[name]; ok {
			t.Errorf("expected %s to be omitted, got %v", name, params[name])
		}
	}
}

func TestUserService_Update_SendsHomeCreate(t *testing.T) {
	caller := &recordingCaller{response: userResponseJSON}

	if _, err := NewUserService(caller).Update(context.Background(), 71, UpdateUserOpts{
		Home:       "/mnt/tank/staff",
		HomeCreate: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params, ok := callArgs(t, caller, 0)[1].(map[string]any)
	if !ok {
		t.Fatal("expected map parameters")
	}
	if params["home_create"] != true {
		t.Errorf("expected home_create true, got %v", params["home_create"])
	}
}

func TestUserService_Update_Error(t *testing.T) {
	caller := &recordingCaller{err: errors.New("connection refused")}

	if _, err := NewUserService(caller).Update(context.Background(), 71, UpdateUserOpts{}); err == nil {
		t.Fatal("expected the API error to be returned")
	}
}

func TestUserService_Update_MalformedResponse(t *testing.T) {
	caller := &recordingCaller{response: `not json`}

	if _, err := NewUserService(caller).Update(context.Background(), 71, UpdateUserOpts{}); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestUserService_Delete(t *testing.T) {
	caller := &recordingCaller{response: `71`}

	if err := NewUserService(caller).Delete(context.Background(), 71, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.call(t, 0).method != "user.delete" {
		t.Errorf("expected method 'user.delete', got %q", caller.call(t, 0).method)
	}

	args := callArgs(t, caller, 0)
	options, ok := args[1].(map[string]any)
	if !ok {
		t.Fatalf("expected map options, got %T", args[1])
	}
	if options["delete_group"] != false {
		t.Errorf("expected delete_group false, got %v", options["delete_group"])
	}
}

func TestUserService_Delete_Error(t *testing.T) {
	caller := &recordingCaller{err: errors.New("connection refused")}

	if err := NewUserService(caller).Delete(context.Background(), 71, true); err == nil {
		t.Fatal("expected the API error to be returned")
	}
}

func TestMockUserService_Defaults(t *testing.T) {
	var api UserServiceAPI = &MockUserService{}
	ctx := context.Background()

	if user, err := api.Create(ctx, CreateUserOpts{}); user != nil || err != nil {
		t.Errorf("expected nil, nil from Create, got %v, %v", user, err)
	}
	if user, err := api.Get(ctx, 1); user != nil || err != nil {
		t.Errorf("expected nil, nil from Get, got %v, %v", user, err)
	}
	if user, err := api.Update(ctx, 1, UpdateUserOpts{}); user != nil || err != nil {
		t.Errorf("expected nil, nil from Update, got %v, %v", user, err)
	}
	if err := api.Delete(ctx, 1, true); err != nil {
		t.Errorf("expected nil from Delete, got %v", err)
	}
}

func TestMockUserService_Delegates(t *testing.T) {
	want := &User{ID: 71}
	api := &MockUserService{
		CreateFunc: func(ctx context.Context, opts CreateUserOpts) (*User, error) { return want, nil },
		GetFunc:    func(ctx context.Context, id int64) (*User, error) { return want, nil },
		UpdateFunc: func(ctx context.Context, id int64, opts UpdateUserOpts) (*User, error) { return want, nil },
		DeleteFunc: func(ctx context.Context, id int64, deleteGroup bool) error { return errors.New("boom") },
	}
	ctx := context.Background()

	if user, _ := api.Create(ctx, CreateUserOpts{}); user != want {
		t.Error("expected Create to delegate to CreateFunc")
	}
	if user, _ := api.Get(ctx, 71); user != want {
		t.Error("expected Get to delegate to GetFunc")
	}
	if user, _ := api.Update(ctx, 71, UpdateUserOpts{}); user != want {
		t.Error("expected Update to delegate to UpdateFunc")
	}
	if err := api.Delete(ctx, 71, true); err == nil {
		t.Error("expected Delete to delegate to DeleteFunc")
	}
}

func TestUserService_AcceptsRealClient(t *testing.T) {
	// The service must accept a client.Client, which is what the provider wires in.
	_ = NewUserService(&client.MockClient{})
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil},
		{name: "missing user", err: errors.New("[ENOENT] None: User 1 does not exist"), want: true},
		{name: "enoent", err: errors.New("[ENOENT] something"), want: true},
		{name: "other", err: errors.New("connection refused")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotFoundError(tc.err); got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
