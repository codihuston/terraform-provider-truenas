package services

import (
	"context"
	"errors"
	"testing"
)

// groupResponseJSON is a group.get_instance payload from TrueNAS SCALE 25.10.
const groupResponseJSON = `{
	"id": 110,
	"gid": 3000,
	"name": "developers",
	"group": "developers",
	"smb": true,
	"sudo_commands": ["/usr/bin/systemctl"],
	"sudo_commands_nopasswd": [],
	"builtin": false
}`

func TestGroupService_Create(t *testing.T) {
	// group.create answers with the entry ID; the service reads the group back.
	caller := &recordingCaller{responses: []string{`110`, groupResponseJSON}}
	gid := int64(3000)

	group, err := NewGroupService(caller).Create(context.Background(), CreateGroupOpts{
		Name:         "developers",
		GID:          &gid,
		SMB:          true,
		SudoCommands: []string{"/usr/bin/systemctl"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.call(t, 0).method != "group.create" {
		t.Errorf("expected method 'group.create', got %q", caller.call(t, 0).method)
	}

	params := callParams(t, caller, 0)
	if params["name"] != "developers" {
		t.Errorf("expected name 'developers', got %v", params["name"])
	}
	if params["gid"] != gid {
		t.Errorf("expected gid 3000, got %v", params["gid"])
	}
	if commands, ok := params["sudo_commands_nopasswd"].([]string); !ok || len(commands) != 0 {
		t.Errorf("expected an empty sudo_commands_nopasswd array, got %v", params["sudo_commands_nopasswd"])
	}

	if group.ID != 110 || group.GID != 3000 {
		t.Errorf("expected group 110/3000, got %d/%d", group.ID, group.GID)
	}
	if len(group.SudoCommands) != 1 {
		t.Errorf("expected one sudo command, got %v", group.SudoCommands)
	}

	if caller.call(t, 1).method != "group.get_instance" {
		t.Errorf("expected the group to be read back, got %q", caller.call(t, 1).method)
	}
	if caller.call(t, 1).params != int64(110) {
		t.Errorf("expected the new entry ID to be read back, got %v", caller.call(t, 1).params)
	}
}

func TestGroupService_Create_OmitsUnsetGID(t *testing.T) {
	caller := &recordingCaller{responses: []string{`110`, groupResponseJSON}}

	if _, err := NewGroupService(caller).Create(context.Background(), CreateGroupOpts{Name: "developers"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := callParams(t, caller, 0)["gid"]; ok {
		t.Error("expected gid to be omitted so the server allocates one")
	}
}

func TestGroupService_Create_Error(t *testing.T) {
	caller := &recordingCaller{err: errors.New("connection refused")}

	if _, err := NewGroupService(caller).Create(context.Background(), CreateGroupOpts{}); err == nil {
		t.Fatal("expected the API error to be returned")
	}
}

func TestGroupService_Create_MalformedResponse(t *testing.T) {
	caller := &recordingCaller{response: `not json`}

	if _, err := NewGroupService(caller).Create(context.Background(), CreateGroupOpts{}); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestGroupService_Create_VanishesBeforeReadBack(t *testing.T) {
	caller := &recordingCaller{
		responses: []string{`110`, ``},
		errs:      []error{nil, errors.New("[ENOENT] None: Group 110 does not exist")},
	}

	if _, err := NewGroupService(caller).Create(context.Background(), CreateGroupOpts{}); err == nil {
		t.Fatal("expected an error when the new group cannot be read back")
	}
}

func TestGroupService_Get(t *testing.T) {
	caller := &recordingCaller{response: groupResponseJSON}

	group, err := NewGroupService(caller).Get(context.Background(), 110)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.call(t, 0).method != "group.get_instance" {
		t.Errorf("expected method 'group.get_instance', got %q", caller.call(t, 0).method)
	}
	if group.Name != "developers" {
		t.Errorf("expected name 'developers', got %q", group.Name)
	}
}

func TestGroupService_Get_NotFound(t *testing.T) {
	caller := &recordingCaller{err: errors.New("[ENOENT] None: Group 99999 does not exist")}

	group, err := NewGroupService(caller).Get(context.Background(), 99999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group != nil {
		t.Errorf("expected a missing group to map to nil, got %+v", group)
	}
}

func TestGroupService_Get_Error(t *testing.T) {
	caller := &recordingCaller{err: errors.New("connection refused")}

	if _, err := NewGroupService(caller).Get(context.Background(), 110); err == nil {
		t.Fatal("expected the API error to be returned")
	}
}

func TestGroupService_Get_MalformedResponse(t *testing.T) {
	caller := &recordingCaller{response: `not json`}

	if _, err := NewGroupService(caller).Get(context.Background(), 110); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestGroupService_Update(t *testing.T) {
	caller := &recordingCaller{responses: []string{`110`, groupResponseJSON}}

	if _, err := NewGroupService(caller).Update(context.Background(), 110, UpdateGroupOpts{
		Name: "engineers",
		SMB:  true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.call(t, 0).method != "group.update" {
		t.Errorf("expected method 'group.update', got %q", caller.call(t, 0).method)
	}

	args := callArgs(t, caller, 0)
	if len(args) != 2 || args[0] != int64(110) {
		t.Fatalf("expected [110, params], got %v", args)
	}

	params, ok := args[1].(map[string]any)
	if !ok {
		t.Fatalf("expected map parameters, got %T", args[1])
	}
	if params["name"] != "engineers" {
		t.Errorf("expected name 'engineers', got %v", params["name"])
	}
	// The API rejects gid on update.
	if _, ok := params["gid"]; ok {
		t.Error("expected gid to be omitted")
	}

	if caller.call(t, 1).method != "group.get_instance" {
		t.Errorf("expected the group to be read back, got %q", caller.call(t, 1).method)
	}
}

func TestGroupService_Update_Error(t *testing.T) {
	caller := &recordingCaller{err: errors.New("connection refused")}

	if _, err := NewGroupService(caller).Update(context.Background(), 110, UpdateGroupOpts{}); err == nil {
		t.Fatal("expected the API error to be returned")
	}
}

func TestGroupService_Update_MalformedResponse(t *testing.T) {
	caller := &recordingCaller{responses: []string{`110`, `not json`}}

	if _, err := NewGroupService(caller).Update(context.Background(), 110, UpdateGroupOpts{}); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestGroupService_Delete(t *testing.T) {
	caller := &recordingCaller{response: `true`}

	if err := NewGroupService(caller).Delete(context.Background(), 110); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caller.call(t, 0).method != "group.delete" {
		t.Errorf("expected method 'group.delete', got %q", caller.call(t, 0).method)
	}

	options, ok := callArgs(t, caller, 0)[1].(map[string]any)
	if !ok {
		t.Fatal("expected map options")
	}
	if options["delete_users"] != false {
		t.Errorf("expected delete_users false, got %v", options["delete_users"])
	}
}

func TestGroupService_Delete_Error(t *testing.T) {
	caller := &recordingCaller{err: errors.New("connection refused")}

	if err := NewGroupService(caller).Delete(context.Background(), 110); err == nil {
		t.Fatal("expected the API error to be returned")
	}
}

func TestGroupService_BuiltinUsersID(t *testing.T) {
	caller := &recordingCaller{response: `[{"id": 91, "name": "builtin_users", "gid": 545}]`}
	service := NewGroupService(caller)

	id, found, err := service.BuiltinUsersID(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found || id != 91 {
		t.Errorf("expected builtin_users 91, got %d (found %v)", id, found)
	}
	if caller.call(t, 0).method != "group.query" {
		t.Errorf("expected method 'group.query', got %q", caller.call(t, 0).method)
	}

	if _, _, err := service.BuiltinUsersID(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(caller.calls) != 1 {
		t.Errorf("expected the lookup to be cached, got %d calls", len(caller.calls))
	}
}

func TestGroupService_BuiltinUsersID_Missing(t *testing.T) {
	caller := &recordingCaller{response: `[]`}

	id, found, err := NewGroupService(caller).BuiltinUsersID(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found || id != 0 {
		t.Errorf("expected no builtin_users group, got %d (found %v)", id, found)
	}
}

func TestGroupService_BuiltinUsersID_Error(t *testing.T) {
	caller := &recordingCaller{err: errors.New("connection refused")}

	if _, _, err := NewGroupService(caller).BuiltinUsersID(context.Background()); err == nil {
		t.Fatal("expected an error to surface")
	}
}

func TestGroupService_BuiltinUsersID_InvalidResponse(t *testing.T) {
	caller := &recordingCaller{response: `not json`}

	if _, _, err := NewGroupService(caller).BuiltinUsersID(context.Background()); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestMockGroupService_Defaults(t *testing.T) {
	var api GroupServiceAPI = &MockGroupService{}
	ctx := context.Background()

	if group, err := api.Create(ctx, CreateGroupOpts{}); group != nil || err != nil {
		t.Errorf("expected nil, nil from Create, got %v, %v", group, err)
	}
	if group, err := api.Get(ctx, 1); group != nil || err != nil {
		t.Errorf("expected nil, nil from Get, got %v, %v", group, err)
	}
	if group, err := api.Update(ctx, 1, UpdateGroupOpts{}); group != nil || err != nil {
		t.Errorf("expected nil, nil from Update, got %v, %v", group, err)
	}
	if err := api.Delete(ctx, 1); err != nil {
		t.Errorf("expected nil from Delete, got %v", err)
	}
	if id, found, err := api.BuiltinUsersID(ctx); id != 0 || found || err != nil {
		t.Errorf("expected 0, false, nil from BuiltinUsersID, got %d, %v, %v", id, found, err)
	}
}

func TestMockGroupService_Delegates(t *testing.T) {
	want := &Group{ID: 110}
	api := &MockGroupService{
		CreateFunc: func(ctx context.Context, opts CreateGroupOpts) (*Group, error) { return want, nil },
		GetFunc:    func(ctx context.Context, id int64) (*Group, error) { return want, nil },
		UpdateFunc: func(ctx context.Context, id int64, opts UpdateGroupOpts) (*Group, error) { return want, nil },
		DeleteFunc: func(ctx context.Context, id int64) error { return errors.New("boom") },
		BuiltinUsersIDFunc: func(ctx context.Context) (int64, bool, error) {
			return 91, true, nil
		},
	}
	ctx := context.Background()

	if group, _ := api.Create(ctx, CreateGroupOpts{}); group != want {
		t.Error("expected Create to delegate to CreateFunc")
	}
	if group, _ := api.Get(ctx, 110); group != want {
		t.Error("expected Get to delegate to GetFunc")
	}
	if group, _ := api.Update(ctx, 110, UpdateGroupOpts{}); group != want {
		t.Error("expected Update to delegate to UpdateFunc")
	}
	if err := api.Delete(ctx, 110); err == nil {
		t.Error("expected Delete to delegate to DeleteFunc")
	}
	if id, found, _ := api.BuiltinUsersID(ctx); id != 91 || !found {
		t.Error("expected BuiltinUsersID to delegate to BuiltinUsersIDFunc")
	}
}
