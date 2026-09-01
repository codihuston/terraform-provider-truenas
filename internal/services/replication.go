package services

import (
	"context"
	"encoding/json"
	"fmt"

	truenas "github.com/deevus/truenas-go"
)

// Replication direction values accepted by the `replication.*` API.
const (
	ReplicationDirectionPush = "PUSH"
)

// Replication transport values accepted by the `replication.*` API.
const (
	ReplicationTransportSSH = "SSH"
)

// Retention policy values accepted by the `replication.*` API.
const (
	ReplicationRetentionSource = "SOURCE"
	ReplicationRetentionCustom = "CUSTOM"
	ReplicationRetentionNone   = "NONE"
)

// ReplicationSchedule is a cron expression plus the daily time window a
// replication task is allowed to run in.
type ReplicationSchedule struct {
	Minute string
	Hour   string
	Dom    string
	Month  string
	Dow    string
	Begin  string
	End    string
}

// ReplicationTask is the user-facing representation of a TrueNAS replication
// task (the `replication` API namespace).
//
// Only the fields the provider manages are modelled. The API exposes further
// options — other transports, pull replication, encryption, multi-tier
// retention — which the provider does not send and therefore does not read
// back; the server keeps them at its own defaults.
type ReplicationTask struct {
	ID                      int64
	Name                    string
	Direction               string
	Transport               string
	SSHCredentials          *int64
	Sudo                    bool
	SourceDatasets          []string
	TargetDataset           string
	Recursive               bool
	Exclude                 []string
	AlsoIncludeNamingSchema []string
	Auto                    bool
	Schedule                *ReplicationSchedule
	RetentionPolicy         string
	LifetimeValue           *int64
	LifetimeUnit            *string
	Readonly                string
	AllowFromScratch        bool
	Compression             *string
	SpeedLimit              *int64
	Retries                 int64
	LoggingLevel            *string
	Enabled                 bool
	// State is the last known run state reported by the server, such as
	// PENDING, RUNNING, FINISHED or ERROR. It is read-only.
	State string
}

// CreateReplicationTaskOpts contains options for creating a replication task.
// All fields are always sent on create.
type CreateReplicationTaskOpts struct {
	Name                    string
	Direction               string
	Transport               string
	SSHCredentials          *int64
	Sudo                    bool
	SourceDatasets          []string
	TargetDataset           string
	Recursive               bool
	Exclude                 []string
	AlsoIncludeNamingSchema []string
	Auto                    bool
	Schedule                *ReplicationSchedule
	RetentionPolicy         string
	LifetimeValue           *int64
	LifetimeUnit            *string
	Readonly                string
	AllowFromScratch        bool
	Compression             *string
	SpeedLimit              *int64
	Retries                 int64
	LoggingLevel            *string
	Enabled                 bool
}

// UpdateReplicationTaskOpts contains options for updating a replication task.
// All fields are always sent on update.
type UpdateReplicationTaskOpts = CreateReplicationTaskOpts

// replicationScheduleWire is the cron object the API accepts and returns for
// both `schedule` and `restrict_schedule`.
type replicationScheduleWire struct {
	Minute string `json:"minute"`
	Hour   string `json:"hour"`
	Dom    string `json:"dom"`
	Month  string `json:"month"`
	Dow    string `json:"dow"`
	Begin  string `json:"begin"`
	End    string `json:"end"`
}

// replicationTaskResponse is the wire format returned by the replication.*
// methods. `ssh_credentials` is submitted as a bare ID but read back as the
// full keychain credential object, and `state` is a nested run-state object.
type replicationTaskResponse struct {
	ID                      int64                    `json:"id"`
	Name                    string                   `json:"name"`
	Direction               string                   `json:"direction"`
	Transport               string                   `json:"transport"`
	SSHCredentials          *replicationCredentialID `json:"ssh_credentials"`
	Sudo                    bool                     `json:"sudo"`
	SourceDatasets          []string                 `json:"source_datasets"`
	TargetDataset           string                   `json:"target_dataset"`
	Recursive               bool                     `json:"recursive"`
	Exclude                 []string                 `json:"exclude"`
	AlsoIncludeNamingSchema []string                 `json:"also_include_naming_schema"`
	Auto                    bool                     `json:"auto"`
	Schedule                *replicationScheduleWire `json:"schedule"`
	RetentionPolicy         string                   `json:"retention_policy"`
	LifetimeValue           *int64                   `json:"lifetime_value"`
	LifetimeUnit            *string                  `json:"lifetime_unit"`
	Readonly                string                   `json:"readonly"`
	AllowFromScratch        bool                     `json:"allow_from_scratch"`
	Compression             *string                  `json:"compression"`
	SpeedLimit              *int64                   `json:"speed_limit"`
	Retries                 int64                    `json:"retries"`
	LoggingLevel            *string                  `json:"logging_level"`
	Enabled                 bool                     `json:"enabled"`
	State                   *replicationStateWire    `json:"state"`
}

// replicationCredentialID picks the ID out of the embedded keychain credential
// object the API returns for `ssh_credentials`.
type replicationCredentialID struct {
	ID int64 `json:"id"`
}

// replicationStateWire is the nested run-state object the API returns.
type replicationStateWire struct {
	State string `json:"state"`
}

// ReplicationService provides typed methods for the replication.* API namespace.
type ReplicationService struct {
	client truenas.Caller
}

// NewReplicationService creates a new ReplicationService.
func NewReplicationService(c truenas.Caller) *ReplicationService {
	return &ReplicationService{client: c}
}

// Create creates a replication task and returns the full object.
func (s *ReplicationService) Create(ctx context.Context, opts CreateReplicationTaskOpts) (*ReplicationTask, error) {
	result, err := s.client.Call(ctx, "replication.create", replicationOptsToParams(opts))
	if err != nil {
		return nil, err
	}

	return replicationTaskFromJSON(result, "create")
}

// Get returns a replication task by ID, or nil if it does not exist.
func (s *ReplicationService) Get(ctx context.Context, id int64) (*ReplicationTask, error) {
	result, err := s.client.Call(ctx, "replication.get_instance", id)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	return replicationTaskFromJSON(result, "get_instance")
}

// List returns all replication tasks.
func (s *ReplicationService) List(ctx context.Context) ([]ReplicationTask, error) {
	result, err := s.client.Call(ctx, "replication.query", nil)
	if err != nil {
		return nil, err
	}

	var responses []replicationTaskResponse
	if err := json.Unmarshal(result, &responses); err != nil {
		return nil, fmt.Errorf("parse query response: %w", err)
	}

	tasks := make([]ReplicationTask, len(responses))
	for i, resp := range responses {
		tasks[i] = replicationTaskFromResponse(resp)
	}
	return tasks, nil
}

// Update updates a replication task and returns the full object.
func (s *ReplicationService) Update(ctx context.Context, id int64, opts UpdateReplicationTaskOpts) (*ReplicationTask, error) {
	result, err := s.client.Call(ctx, "replication.update", []any{id, replicationOptsToParams(opts)})
	if err != nil {
		return nil, err
	}

	return replicationTaskFromJSON(result, "update")
}

// Delete deletes a replication task by ID.
func (s *ReplicationService) Delete(ctx context.Context, id int64) error {
	_, err := s.client.Call(ctx, "replication.delete", id)
	return err
}

// replicationOptsToParams converts CreateReplicationTaskOpts to API parameters.
func replicationOptsToParams(opts CreateReplicationTaskOpts) map[string]any {
	return map[string]any{
		"name":                       opts.Name,
		"direction":                  opts.Direction,
		"transport":                  opts.Transport,
		"ssh_credentials":            opts.SSHCredentials,
		"sudo":                       opts.Sudo,
		"source_datasets":            stringList(opts.SourceDatasets),
		"target_dataset":             opts.TargetDataset,
		"recursive":                  opts.Recursive,
		"exclude":                    stringList(opts.Exclude),
		"also_include_naming_schema": stringList(opts.AlsoIncludeNamingSchema),
		"auto":                       opts.Auto,
		"schedule":                   replicationScheduleToParams(opts.Schedule),
		"retention_policy":           opts.RetentionPolicy,
		"lifetime_value":             opts.LifetimeValue,
		"lifetime_unit":              opts.LifetimeUnit,
		"readonly":                   opts.Readonly,
		"allow_from_scratch":         opts.AllowFromScratch,
		"compression":                opts.Compression,
		"speed_limit":                opts.SpeedLimit,
		"retries":                    opts.Retries,
		"logging_level":              opts.LoggingLevel,
		"enabled":                    opts.Enabled,
	}
}

// replicationScheduleToParams renders the cron object, or nil for a task that
// does not run on a schedule.
func replicationScheduleToParams(schedule *ReplicationSchedule) any {
	if schedule == nil {
		return nil
	}

	return map[string]any{
		"minute": schedule.Minute,
		"hour":   schedule.Hour,
		"dom":    schedule.Dom,
		"month":  schedule.Month,
		"dow":    schedule.Dow,
		"begin":  schedule.Begin,
		"end":    schedule.End,
	}
}

// replicationTaskFromJSON decodes a single-task response body. method names the
// API call for the error message.
func replicationTaskFromJSON(body []byte, method string) (*ReplicationTask, error) {
	var resp replicationTaskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse %s response: %w", method, err)
	}

	task := replicationTaskFromResponse(resp)
	return &task, nil
}

// replicationTaskFromResponse converts a wire-format response to a user-facing
// ReplicationTask.
func replicationTaskFromResponse(resp replicationTaskResponse) ReplicationTask {
	task := ReplicationTask{
		ID:                      resp.ID,
		Name:                    resp.Name,
		Direction:               resp.Direction,
		Transport:               resp.Transport,
		Sudo:                    resp.Sudo,
		SourceDatasets:          stringList(resp.SourceDatasets),
		TargetDataset:           resp.TargetDataset,
		Recursive:               resp.Recursive,
		Exclude:                 stringList(resp.Exclude),
		AlsoIncludeNamingSchema: stringList(resp.AlsoIncludeNamingSchema),
		Auto:                    resp.Auto,
		RetentionPolicy:         resp.RetentionPolicy,
		LifetimeValue:           resp.LifetimeValue,
		LifetimeUnit:            resp.LifetimeUnit,
		Readonly:                resp.Readonly,
		AllowFromScratch:        resp.AllowFromScratch,
		Compression:             resp.Compression,
		SpeedLimit:              resp.SpeedLimit,
		Retries:                 resp.Retries,
		LoggingLevel:            resp.LoggingLevel,
		Enabled:                 resp.Enabled,
	}

	if resp.SSHCredentials != nil {
		id := resp.SSHCredentials.ID
		task.SSHCredentials = &id
	}

	if resp.Schedule != nil {
		task.Schedule = &ReplicationSchedule{
			Minute: resp.Schedule.Minute,
			Hour:   resp.Schedule.Hour,
			Dom:    resp.Schedule.Dom,
			Month:  resp.Schedule.Month,
			Dow:    resp.Schedule.Dow,
			Begin:  resp.Schedule.Begin,
			End:    resp.Schedule.End,
		}
	}

	if resp.State != nil {
		task.State = resp.State.State
	}

	return task
}
