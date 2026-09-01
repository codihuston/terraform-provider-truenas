package services

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// testReplicationTaskJSON is a replication.create response captured from
// TrueNAS SCALE 25.10.6. `ssh_credentials` comes back as the full keychain
// credential object, not the ID that was submitted.
const testReplicationTaskJSON = `{
	"id": 1,
	"name": "archive-to-backup",
	"direction": "PUSH",
	"transport": "SSH",
	"ssh_credentials": {
		"id": 2,
		"name": "backup-host",
		"type": "SSH_CREDENTIALS",
		"attributes": {"host": "backup.example.com", "port": 22, "username": "root"}
	},
	"sudo": false,
	"source_datasets": ["tank/archive"],
	"target_dataset": "tank/backup",
	"recursive": true,
	"exclude": ["tank/archive/scratch"],
	"also_include_naming_schema": ["auto-%Y-%m-%d_%H-%M"],
	"auto": true,
	"schedule": {
		"minute": "0",
		"hour": "3",
		"dom": "*",
		"month": "*",
		"dow": "*",
		"begin": "00:00",
		"end": "23:59"
	},
	"retention_policy": "CUSTOM",
	"lifetime_value": 2,
	"lifetime_unit": "WEEK",
	"readonly": "SET",
	"allow_from_scratch": false,
	"compression": "LZ4",
	"speed_limit": 1048576,
	"retries": 5,
	"logging_level": null,
	"enabled": true,
	"state": {"state": "PENDING"}
}`

func testReplicationOpts() CreateReplicationTaskOpts {
	credential := int64(2)
	lifetimeValue := int64(2)
	lifetimeUnit := "WEEK"

	return CreateReplicationTaskOpts{
		Name:                    "archive-to-backup",
		Direction:               ReplicationDirectionPush,
		Transport:               ReplicationTransportSSH,
		SSHCredentials:          &credential,
		SourceDatasets:          []string{"tank/archive"},
		TargetDataset:           "tank/backup",
		Recursive:               true,
		AlsoIncludeNamingSchema: []string{"auto-%Y-%m-%d_%H-%M"},
		Auto:                    true,
		Schedule: &ReplicationSchedule{
			Minute: "0", Hour: "3", Dom: "*", Month: "*", Dow: "*",
			Begin: "00:00", End: "23:59",
		},
		RetentionPolicy: ReplicationRetentionCustom,
		LifetimeValue:   &lifetimeValue,
		LifetimeUnit:    &lifetimeUnit,
		Readonly:        "SET",
		Retries:         5,
		Enabled:         true,
	}
}

func assertReplicationTask(t *testing.T, task *ReplicationTask) {
	t.Helper()

	if task == nil {
		t.Fatal("expected a task")
	}
	if task.ID != 1 {
		t.Errorf("expected ID 1, got %d", task.ID)
	}
	if task.Name != "archive-to-backup" {
		t.Errorf("expected name 'archive-to-backup', got %q", task.Name)
	}
	if task.SSHCredentials == nil || *task.SSHCredentials != 2 {
		t.Errorf("expected SSHCredentials 2, got %v", task.SSHCredentials)
	}
	if !reflect.DeepEqual(task.SourceDatasets, []string{"tank/archive"}) {
		t.Errorf("unexpected source datasets: %v", task.SourceDatasets)
	}
	if !reflect.DeepEqual(task.Exclude, []string{"tank/archive/scratch"}) {
		t.Errorf("unexpected exclude: %v", task.Exclude)
	}
	if task.Schedule == nil || task.Schedule.Hour != "3" || task.Schedule.End != "23:59" {
		t.Errorf("unexpected schedule: %+v", task.Schedule)
	}
	if task.LifetimeValue == nil || *task.LifetimeValue != 2 {
		t.Errorf("expected LifetimeValue 2, got %v", task.LifetimeValue)
	}
	if task.LifetimeUnit == nil || *task.LifetimeUnit != "WEEK" {
		t.Errorf("expected LifetimeUnit WEEK, got %v", task.LifetimeUnit)
	}
	if task.Compression == nil || *task.Compression != "LZ4" {
		t.Errorf("expected Compression LZ4, got %v", task.Compression)
	}
	if task.SpeedLimit == nil || *task.SpeedLimit != 1048576 {
		t.Errorf("expected SpeedLimit 1048576, got %v", task.SpeedLimit)
	}
	if task.LoggingLevel != nil {
		t.Errorf("expected nil LoggingLevel, got %v", *task.LoggingLevel)
	}
	if task.State != "PENDING" {
		t.Errorf("expected state PENDING, got %q", task.State)
	}
}

func TestReplicationService_Create(t *testing.T) {
	c := &fakeCaller{result: testReplicationTaskJSON}
	s := NewReplicationService(c)

	task, err := s.Create(context.Background(), testReplicationOpts())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if c.method != "replication.create" {
		t.Errorf("expected replication.create, got %q", c.method)
	}

	params, ok := c.params.(map[string]any)
	if !ok {
		t.Fatalf("expected map params, got %T", c.params)
	}
	if params["name"] != "archive-to-backup" {
		t.Errorf("unexpected name param: %v", params["name"])
	}
	if params["auto"] != true {
		t.Errorf("expected auto true, got %v", params["auto"])
	}
	schedule, ok := params["schedule"].(map[string]any)
	if !ok {
		t.Fatalf("expected schedule map, got %T", params["schedule"])
	}
	if schedule["hour"] != "3" || schedule["begin"] != "00:00" {
		t.Errorf("unexpected schedule params: %v", schedule)
	}

	assertReplicationTask(t, task)
}

func TestReplicationService_Create_NilSlicesAndScheduleBecomeAPIDefaults(t *testing.T) {
	c := &fakeCaller{result: testReplicationTaskJSON}
	s := NewReplicationService(c)

	if _, err := s.Create(context.Background(), CreateReplicationTaskOpts{}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	params := c.params.(map[string]any)
	for _, key := range []string{"source_datasets", "exclude", "also_include_naming_schema"} {
		got, ok := params[key].([]string)
		if !ok {
			t.Fatalf("expected []string for %s, got %T", key, params[key])
		}
		if got == nil || len(got) != 0 {
			t.Errorf("expected empty non-nil slice for %s, got %v", key, got)
		}
	}
	if params["schedule"] != nil {
		t.Errorf("expected nil schedule, got %v", params["schedule"])
	}
}

func TestReplicationService_Create_Error(t *testing.T) {
	s := NewReplicationService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.Create(context.Background(), testReplicationOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestReplicationService_Create_MalformedResponse(t *testing.T) {
	s := NewReplicationService(&fakeCaller{result: `not json`})

	if _, err := s.Create(context.Background(), testReplicationOpts()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestReplicationService_Get(t *testing.T) {
	c := &fakeCaller{result: testReplicationTaskJSON}
	s := NewReplicationService(c)

	task, err := s.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.method != "replication.get_instance" {
		t.Errorf("expected replication.get_instance, got %q", c.method)
	}
	if c.params != int64(1) {
		t.Errorf("expected params 1, got %v", c.params)
	}

	assertReplicationTask(t, task)
}

func TestReplicationService_Get_NotFound(t *testing.T) {
	s := NewReplicationService(&fakeCaller{err: enoentRPCError()})

	task, err := s.Get(context.Background(), 99999)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if task != nil {
		t.Errorf("expected nil task, got %+v", task)
	}
}

func TestReplicationService_Get_Error(t *testing.T) {
	s := NewReplicationService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.Get(context.Background(), 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestReplicationService_Get_MalformedResponse(t *testing.T) {
	s := NewReplicationService(&fakeCaller{result: `[]`})

	if _, err := s.Get(context.Background(), 1); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestReplicationService_List(t *testing.T) {
	c := &fakeCaller{result: "[" + testReplicationTaskJSON + "]"}
	s := NewReplicationService(c)

	tasks, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.method != "replication.query" {
		t.Errorf("expected replication.query, got %q", c.method)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	assertReplicationTask(t, &tasks[0])
}

func TestReplicationService_List_Error(t *testing.T) {
	s := NewReplicationService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestReplicationService_List_MalformedResponse(t *testing.T) {
	s := NewReplicationService(&fakeCaller{result: `{}`})

	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestReplicationService_Update(t *testing.T) {
	c := &fakeCaller{result: testReplicationTaskJSON}
	s := NewReplicationService(c)

	task, err := s.Update(context.Background(), 1, testReplicationOpts())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.method != "replication.update" {
		t.Errorf("expected replication.update, got %q", c.method)
	}

	args, ok := c.params.([]any)
	if !ok || len(args) != 2 {
		t.Fatalf("expected [id, params], got %v", c.params)
	}
	if args[0] != int64(1) {
		t.Errorf("expected id 1, got %v", args[0])
	}

	assertReplicationTask(t, task)
}

func TestReplicationService_Update_Error(t *testing.T) {
	s := NewReplicationService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.Update(context.Background(), 1, testReplicationOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestReplicationService_Update_MalformedResponse(t *testing.T) {
	s := NewReplicationService(&fakeCaller{result: `[]`})

	if _, err := s.Update(context.Background(), 1, testReplicationOpts()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestReplicationService_Delete(t *testing.T) {
	c := &fakeCaller{result: `null`}
	s := NewReplicationService(c)

	if err := s.Delete(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.method != "replication.delete" {
		t.Errorf("expected replication.delete, got %q", c.method)
	}
	if c.params != int64(1) {
		t.Errorf("expected params 1, got %v", c.params)
	}
}

func TestReplicationService_Delete_Error(t *testing.T) {
	s := NewReplicationService(&fakeCaller{err: errors.New("boom")})

	if err := s.Delete(context.Background(), 1); err == nil {
		t.Fatal("expected error")
	}
}

// TestReplicationTaskFromResponse_AbsentOptionalObjects covers a task the API
// returns without a schedule, credential or run state: a manual LOCAL-style
// task read back through the same decoder.
func TestReplicationTaskFromResponse_AbsentOptionalObjects(t *testing.T) {
	var resp replicationTaskResponse
	if err := json.Unmarshal([]byte(`{
		"id": 4,
		"name": "manual",
		"ssh_credentials": null,
		"schedule": null,
		"state": null,
		"source_datasets": null,
		"exclude": null,
		"also_include_naming_schema": null
	}`), &resp); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	task := replicationTaskFromResponse(resp)

	if task.SSHCredentials != nil {
		t.Error("expected nil SSHCredentials")
	}
	if task.Schedule != nil {
		t.Error("expected nil Schedule")
	}
	if task.State != "" {
		t.Errorf("expected empty state, got %q", task.State)
	}
	for name, got := range map[string][]string{
		"SourceDatasets":          task.SourceDatasets,
		"Exclude":                 task.Exclude,
		"AlsoIncludeNamingSchema": task.AlsoIncludeNamingSchema,
	} {
		if got == nil || len(got) != 0 {
			t.Errorf("expected empty non-nil slice for %s, got %v", name, got)
		}
	}
}

func TestMockReplicationService_Defaults(t *testing.T) {
	var m ReplicationServiceAPI = &MockReplicationService{}
	ctx := context.Background()

	if task, err := m.Create(ctx, CreateReplicationTaskOpts{}); task != nil || err != nil {
		t.Error("expected nil, nil from default Create")
	}
	if task, err := m.Get(ctx, 1); task != nil || err != nil {
		t.Error("expected nil, nil from default Get")
	}
	if tasks, err := m.List(ctx); tasks != nil || err != nil {
		t.Error("expected nil, nil from default List")
	}
	if task, err := m.Update(ctx, 1, UpdateReplicationTaskOpts{}); task != nil || err != nil {
		t.Error("expected nil, nil from default Update")
	}
	if err := m.Delete(ctx, 1); err != nil {
		t.Error("expected nil from default Delete")
	}
}

func TestMockReplicationService_Overrides(t *testing.T) {
	want := &ReplicationTask{ID: 3}
	m := &MockReplicationService{
		CreateFunc: func(ctx context.Context, opts CreateReplicationTaskOpts) (*ReplicationTask, error) {
			return want, nil
		},
		GetFunc:  func(ctx context.Context, id int64) (*ReplicationTask, error) { return want, nil },
		ListFunc: func(ctx context.Context) ([]ReplicationTask, error) { return []ReplicationTask{*want}, nil },
		UpdateFunc: func(ctx context.Context, id int64, opts UpdateReplicationTaskOpts) (*ReplicationTask, error) {
			return want, nil
		},
		DeleteFunc: func(ctx context.Context, id int64) error { return errors.New("boom") },
	}
	ctx := context.Background()

	if got, _ := m.Create(ctx, CreateReplicationTaskOpts{}); got != want {
		t.Error("CreateFunc not used")
	}
	if got, _ := m.Get(ctx, 1); got != want {
		t.Error("GetFunc not used")
	}
	if got, _ := m.List(ctx); len(got) != 1 {
		t.Error("ListFunc not used")
	}
	if got, _ := m.Update(ctx, 1, UpdateReplicationTaskOpts{}); got != want {
		t.Error("UpdateFunc not used")
	}
	if err := m.Delete(ctx, 1); err == nil {
		t.Error("DeleteFunc not used")
	}
}
