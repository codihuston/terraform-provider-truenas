package services

import (
	"context"
	"encoding/json"
	"fmt"

	truenas "github.com/deevus/truenas-go"
)

// SnapshotSchedule is the cron schedule a periodic snapshot task runs on.
// Every field is a cron expression except Begin and End, which bound the
// wall-clock window ("HH:MM") in which the task is allowed to fire.
type SnapshotSchedule struct {
	Minute string
	Hour   string
	Dom    string
	Month  string
	Dow    string
	Begin  string
	End    string
}

// SnapshotTask is the user-facing representation of a TrueNAS periodic
// snapshot task (the `pool.snapshottask` API namespace).
type SnapshotTask struct {
	ID            int64
	Dataset       string
	Recursive     bool
	Exclude       []string
	LifetimeValue int64
	LifetimeUnit  string
	NamingSchema  string
	AllowEmpty    bool
	Enabled       bool
	Schedule      SnapshotSchedule
	// VMwareSync reports whether VMware virtual machines are synchronised
	// before snapshots are taken. It is derived from the VMware-snapshot
	// configuration rather than set on the task.
	VMwareSync bool
}

// CreateSnapshotTaskOpts contains options for creating a periodic snapshot
// task. All fields are always sent on create.
type CreateSnapshotTaskOpts struct {
	Dataset       string
	Recursive     bool
	Exclude       []string
	LifetimeValue int64
	LifetimeUnit  string
	NamingSchema  string
	AllowEmpty    bool
	Enabled       bool
	Schedule      SnapshotSchedule
}

// UpdateSnapshotTaskOpts contains options for updating a periodic snapshot
// task. All fields are always sent on update.
type UpdateSnapshotTaskOpts = CreateSnapshotTaskOpts

// snapshotTaskResponse is the wire format returned by the
// pool.snapshottask.* methods.
type snapshotTaskResponse struct {
	ID            int64                    `json:"id"`
	Dataset       string                   `json:"dataset"`
	Recursive     bool                     `json:"recursive"`
	Exclude       []string                 `json:"exclude"`
	LifetimeValue int64                    `json:"lifetime_value"`
	LifetimeUnit  string                   `json:"lifetime_unit"`
	NamingSchema  string                   `json:"naming_schema"`
	AllowEmpty    bool                     `json:"allow_empty"`
	Enabled       bool                     `json:"enabled"`
	Schedule      snapshotScheduleResponse `json:"schedule"`
	VMwareSync    bool                     `json:"vmware_sync"`
}

type snapshotScheduleResponse struct {
	Minute string `json:"minute"`
	Hour   string `json:"hour"`
	Dom    string `json:"dom"`
	Month  string `json:"month"`
	Dow    string `json:"dow"`
	Begin  string `json:"begin"`
	End    string `json:"end"`
}

// SnapshotTaskService provides typed methods for the pool.snapshottask.* API namespace.
type SnapshotTaskService struct {
	client truenas.Caller
}

// NewSnapshotTaskService creates a new SnapshotTaskService.
func NewSnapshotTaskService(c truenas.Caller) *SnapshotTaskService {
	return &SnapshotTaskService{client: c}
}

// Create creates a periodic snapshot task and returns the full object.
func (s *SnapshotTaskService) Create(ctx context.Context, opts CreateSnapshotTaskOpts) (*SnapshotTask, error) {
	result, err := s.client.Call(ctx, "pool.snapshottask.create", snapshotTaskOptsToParams(opts))
	if err != nil {
		return nil, err
	}

	var resp snapshotTaskResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}

	task := snapshotTaskFromResponse(resp)
	return &task, nil
}

// Get returns a periodic snapshot task by ID, or nil if it does not exist.
func (s *SnapshotTaskService) Get(ctx context.Context, id int64) (*SnapshotTask, error) {
	result, err := s.client.Call(ctx, "pool.snapshottask.get_instance", id)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	var resp snapshotTaskResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse get_instance response: %w", err)
	}

	task := snapshotTaskFromResponse(resp)
	return &task, nil
}

// List returns all periodic snapshot tasks.
func (s *SnapshotTaskService) List(ctx context.Context) ([]SnapshotTask, error) {
	result, err := s.client.Call(ctx, "pool.snapshottask.query", nil)
	if err != nil {
		return nil, err
	}

	var responses []snapshotTaskResponse
	if err := json.Unmarshal(result, &responses); err != nil {
		return nil, fmt.Errorf("parse query response: %w", err)
	}

	tasks := make([]SnapshotTask, len(responses))
	for i, resp := range responses {
		tasks[i] = snapshotTaskFromResponse(resp)
	}
	return tasks, nil
}

// Update updates a periodic snapshot task and returns the full object.
func (s *SnapshotTaskService) Update(ctx context.Context, id int64, opts UpdateSnapshotTaskOpts) (*SnapshotTask, error) {
	result, err := s.client.Call(ctx, "pool.snapshottask.update", []any{id, snapshotTaskOptsToParams(opts)})
	if err != nil {
		return nil, err
	}

	var resp snapshotTaskResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse update response: %w", err)
	}

	task := snapshotTaskFromResponse(resp)
	return &task, nil
}

// Delete deletes a periodic snapshot task by ID. Snapshots the task already
// took are left in place and keep the expiry their naming schema implies.
func (s *SnapshotTaskService) Delete(ctx context.Context, id int64) error {
	_, err := s.client.Call(ctx, "pool.snapshottask.delete", id)
	return err
}

// snapshotTaskOptsToParams converts CreateSnapshotTaskOpts to API parameters.
func snapshotTaskOptsToParams(opts CreateSnapshotTaskOpts) map[string]any {
	return map[string]any{
		"dataset":        opts.Dataset,
		"recursive":      opts.Recursive,
		"exclude":        stringList(opts.Exclude),
		"lifetime_value": opts.LifetimeValue,
		"lifetime_unit":  opts.LifetimeUnit,
		"naming_schema":  opts.NamingSchema,
		"allow_empty":    opts.AllowEmpty,
		"enabled":        opts.Enabled,
		"schedule": map[string]any{
			"minute": opts.Schedule.Minute,
			"hour":   opts.Schedule.Hour,
			"dom":    opts.Schedule.Dom,
			"month":  opts.Schedule.Month,
			"dow":    opts.Schedule.Dow,
			"begin":  opts.Schedule.Begin,
			"end":    opts.Schedule.End,
		},
	}
}

// snapshotTaskFromResponse converts a wire-format response to a user-facing SnapshotTask.
func snapshotTaskFromResponse(resp snapshotTaskResponse) SnapshotTask {
	return SnapshotTask{
		ID:            resp.ID,
		Dataset:       resp.Dataset,
		Recursive:     resp.Recursive,
		Exclude:       stringList(resp.Exclude),
		LifetimeValue: resp.LifetimeValue,
		LifetimeUnit:  resp.LifetimeUnit,
		NamingSchema:  resp.NamingSchema,
		AllowEmpty:    resp.AllowEmpty,
		Enabled:       resp.Enabled,
		Schedule: SnapshotSchedule{
			Minute: resp.Schedule.Minute,
			Hour:   resp.Schedule.Hour,
			Dom:    resp.Schedule.Dom,
			Month:  resp.Schedule.Month,
			Dow:    resp.Schedule.Dow,
			Begin:  resp.Schedule.Begin,
			End:    resp.Schedule.End,
		},
		VMwareSync: resp.VMwareSync,
	}
}
