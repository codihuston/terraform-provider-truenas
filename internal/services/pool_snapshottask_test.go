package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// testSnapshotTaskJSON is the pool.snapshottask.create response the live API
// returns for testSnapshotTaskOpts.
const testSnapshotTaskJSON = `{
	"dataset": "tank/data",
	"recursive": true,
	"lifetime_value": 4,
	"lifetime_unit": "WEEK",
	"enabled": true,
	"exclude": ["tank/data/scratch"],
	"naming_schema": "nightly-%Y-%m-%d_%H-%M",
	"allow_empty": true,
	"schedule": {
		"minute": "0",
		"hour": "2",
		"dom": "*",
		"month": "*",
		"dow": "*",
		"begin": "00:00",
		"end": "23:59"
	},
	"id": 3,
	"state": {"state": "PENDING"},
	"vmware_sync": false
}`

func testSnapshotTaskOpts() CreateSnapshotTaskOpts {
	return CreateSnapshotTaskOpts{
		Dataset:       "tank/data",
		Recursive:     true,
		Exclude:       []string{"tank/data/scratch"},
		LifetimeValue: 4,
		LifetimeUnit:  "WEEK",
		NamingSchema:  "nightly-%Y-%m-%d_%H-%M",
		AllowEmpty:    true,
		Enabled:       true,
		Schedule: SnapshotSchedule{
			Minute: "0",
			Hour:   "2",
			Dom:    "*",
			Month:  "*",
			Dow:    "*",
			Begin:  "00:00",
			End:    "23:59",
		},
	}
}

func assertSnapshotTask(t *testing.T, task *SnapshotTask) {
	t.Helper()

	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if task.ID != 3 {
		t.Errorf("expected ID 3, got %d", task.ID)
	}
	if task.Dataset != "tank/data" {
		t.Errorf("expected dataset 'tank/data', got %q", task.Dataset)
	}
	if !task.Recursive {
		t.Error("expected recursive true")
	}
	if task.LifetimeValue != 4 || task.LifetimeUnit != "WEEK" {
		t.Errorf("unexpected lifetime: %d %s", task.LifetimeValue, task.LifetimeUnit)
	}
	if task.NamingSchema != "nightly-%Y-%m-%d_%H-%M" {
		t.Errorf("unexpected naming_schema: %q", task.NamingSchema)
	}
	if !task.AllowEmpty {
		t.Error("expected allow_empty true")
	}
	if !task.Enabled {
		t.Error("expected enabled true")
	}
	if task.VMwareSync {
		t.Error("expected vmware_sync false")
	}
	if !reflect.DeepEqual(task.Exclude, []string{"tank/data/scratch"}) {
		t.Errorf("unexpected exclude: %v", task.Exclude)
	}
	if want := testSnapshotTaskOpts().Schedule; task.Schedule != want {
		t.Errorf("expected schedule %+v, got %+v", want, task.Schedule)
	}
}

func TestSnapshotTaskService_Create(t *testing.T) {
	c := &fakeCaller{result: testSnapshotTaskJSON}
	s := NewSnapshotTaskService(c)

	task, err := s.Create(context.Background(), testSnapshotTaskOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "pool.snapshottask.create" {
		t.Errorf("expected method 'pool.snapshottask.create', got %q", c.method)
	}

	params, ok := c.params.(map[string]any)
	if !ok {
		t.Fatalf("expected map params, got %T", c.params)
	}
	if params["dataset"] != "tank/data" {
		t.Errorf("unexpected dataset param: %v", params["dataset"])
	}
	if params["lifetime_value"] != int64(4) {
		t.Errorf("unexpected lifetime_value param: %v", params["lifetime_value"])
	}
	// state and vmware_sync are read-only and must never be submitted.
	for _, key := range []string{"id", "state", "vmware_sync"} {
		if _, ok := params[key]; ok {
			t.Errorf("expected no %s param", key)
		}
	}

	schedule, ok := params["schedule"].(map[string]any)
	if !ok {
		t.Fatalf("expected map schedule param, got %T", params["schedule"])
	}
	for key, want := range map[string]string{
		"minute": "0", "hour": "2", "dom": "*", "month": "*", "dow": "*",
		"begin": "00:00", "end": "23:59",
	} {
		if schedule[key] != want {
			t.Errorf("expected schedule.%s %q, got %v", key, want, schedule[key])
		}
	}

	assertSnapshotTask(t, task)
}

// A nil exclude must be sent as [] rather than null, which the API rejects.
func TestSnapshotTaskOptsToParams_NilListBecomesEmpty(t *testing.T) {
	params := snapshotTaskOptsToParams(CreateSnapshotTaskOpts{Dataset: "tank/data"})

	if !reflect.DeepEqual(params["exclude"], []string{}) {
		t.Errorf("expected empty exclude param, got %v", params["exclude"])
	}
}

func TestSnapshotTaskService_Create_CallError(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.Create(context.Background(), testSnapshotTaskOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSnapshotTaskService_Create_BadJSON(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{result: `not json`})

	if _, err := s.Create(context.Background(), testSnapshotTaskOpts()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSnapshotTaskService_Get(t *testing.T) {
	c := &fakeCaller{result: testSnapshotTaskJSON}
	s := NewSnapshotTaskService(c)

	task, err := s.Get(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "pool.snapshottask.get_instance" {
		t.Errorf("expected method 'pool.snapshottask.get_instance', got %q", c.method)
	}
	if c.params != int64(3) {
		t.Errorf("expected params 3, got %v", c.params)
	}
	assertSnapshotTask(t, task)
}

func TestSnapshotTaskService_Get_NotFound(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{err: enoentRPCError()})

	task, err := s.Get(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task != nil {
		t.Fatalf("expected nil task, got %+v", task)
	}
}

func TestSnapshotTaskService_Get_OtherError(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.Get(context.Background(), 3); err == nil {
		t.Fatal("expected error")
	}
}

func TestSnapshotTaskService_Get_BadJSON(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{result: `{`})

	if _, err := s.Get(context.Background(), 3); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSnapshotTaskService_List(t *testing.T) {
	c := &fakeCaller{result: `[` + testSnapshotTaskJSON + `]`}
	s := NewSnapshotTaskService(c)

	tasks, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "pool.snapshottask.query" {
		t.Errorf("expected method 'pool.snapshottask.query', got %q", c.method)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	assertSnapshotTask(t, &tasks[0])
}

func TestSnapshotTaskService_List_CallError(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSnapshotTaskService_List_BadJSON(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{result: `{}`})

	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSnapshotTaskService_Update(t *testing.T) {
	c := &fakeCaller{result: testSnapshotTaskJSON}
	s := NewSnapshotTaskService(c)

	task, err := s.Update(context.Background(), 3, testSnapshotTaskOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "pool.snapshottask.update" {
		t.Errorf("expected method 'pool.snapshottask.update', got %q", c.method)
	}

	args, ok := c.params.([]any)
	if !ok || len(args) != 2 {
		t.Fatalf("expected [id, data] params, got %v", c.params)
	}
	if args[0] != int64(3) {
		t.Errorf("expected id 3, got %v", args[0])
	}
	assertSnapshotTask(t, task)
}

func TestSnapshotTaskService_Update_CallError(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.Update(context.Background(), 3, testSnapshotTaskOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSnapshotTaskService_Update_BadJSON(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{result: `[]`})

	if _, err := s.Update(context.Background(), 3, testSnapshotTaskOpts()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSnapshotTaskService_Delete(t *testing.T) {
	c := &fakeCaller{result: `true`}
	s := NewSnapshotTaskService(c)

	if err := s.Delete(context.Background(), 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "pool.snapshottask.delete" {
		t.Errorf("expected method 'pool.snapshottask.delete', got %q", c.method)
	}
	if c.params != int64(3) {
		t.Errorf("expected params 3, got %v", c.params)
	}
}

func TestSnapshotTaskService_Delete_Error(t *testing.T) {
	s := NewSnapshotTaskService(&fakeCaller{err: errors.New("boom")})

	if err := s.Delete(context.Background(), 3); err == nil {
		t.Fatal("expected error")
	}
}

func TestMockSnapshotTaskService_Defaults(t *testing.T) {
	var m SnapshotTaskServiceAPI = &MockSnapshotTaskService{}
	ctx := context.Background()

	if task, err := m.Create(ctx, CreateSnapshotTaskOpts{}); task != nil || err != nil {
		t.Error("expected nil, nil from default Create")
	}
	if task, err := m.Get(ctx, 1); task != nil || err != nil {
		t.Error("expected nil, nil from default Get")
	}
	if tasks, err := m.List(ctx); tasks != nil || err != nil {
		t.Error("expected nil, nil from default List")
	}
	if task, err := m.Update(ctx, 1, UpdateSnapshotTaskOpts{}); task != nil || err != nil {
		t.Error("expected nil, nil from default Update")
	}
	if err := m.Delete(ctx, 1); err != nil {
		t.Error("expected nil from default Delete")
	}
}

func TestMockSnapshotTaskService_Overrides(t *testing.T) {
	want := &SnapshotTask{ID: 3}
	m := &MockSnapshotTaskService{
		CreateFunc: func(ctx context.Context, opts CreateSnapshotTaskOpts) (*SnapshotTask, error) { return want, nil },
		GetFunc:    func(ctx context.Context, id int64) (*SnapshotTask, error) { return want, nil },
		ListFunc:   func(ctx context.Context) ([]SnapshotTask, error) { return []SnapshotTask{*want}, nil },
		UpdateFunc: func(ctx context.Context, id int64, opts UpdateSnapshotTaskOpts) (*SnapshotTask, error) {
			return want, nil
		},
		DeleteFunc: func(ctx context.Context, id int64) error { return errors.New("boom") },
	}
	ctx := context.Background()

	if got, _ := m.Create(ctx, CreateSnapshotTaskOpts{}); got != want {
		t.Error("CreateFunc not used")
	}
	if got, _ := m.Get(ctx, 1); got != want {
		t.Error("GetFunc not used")
	}
	if got, _ := m.List(ctx); len(got) != 1 {
		t.Error("ListFunc not used")
	}
	if got, _ := m.Update(ctx, 1, UpdateSnapshotTaskOpts{}); got != want {
		t.Error("UpdateFunc not used")
	}
	if err := m.Delete(ctx, 1); err == nil {
		t.Error("DeleteFunc not used")
	}
}
