package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestNewReplicationTaskResource(t *testing.T) {
	r := NewReplicationTaskResource()
	if r == nil {
		t.Fatal("NewReplicationTaskResource returned nil")
	}

	if _, ok := r.(*ReplicationTaskResource); !ok {
		t.Fatalf("expected *ReplicationTaskResource, got %T", r)
	}

	_ = resource.Resource(r)
	_ = resource.ResourceWithConfigure(r.(*ReplicationTaskResource))
	_ = resource.ResourceWithImportState(r.(*ReplicationTaskResource))
	_ = resource.ResourceWithValidateConfig(r.(*ReplicationTaskResource))
}

func TestReplicationTaskResource_Metadata(t *testing.T) {
	r := NewReplicationTaskResource()

	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "truenas"}, resp)

	if resp.TypeName != "truenas_replication_task" {
		t.Errorf("expected TypeName 'truenas_replication_task', got %q", resp.TypeName)
	}
}

func TestReplicationTaskResource_Configure_Success(t *testing.T) {
	r := NewReplicationTaskResource().(*ReplicationTaskResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &services.TrueNASServices{}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestReplicationTaskResource_Configure_NilProviderData(t *testing.T) {
	r := NewReplicationTaskResource().(*ReplicationTaskResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestReplicationTaskResource_Configure_WrongType(t *testing.T) {
	r := NewReplicationTaskResource().(*ReplicationTaskResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "not a client"}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for wrong ProviderData type")
	}
}

func TestReplicationTaskResource_Schema(t *testing.T) {
	schemaResp := getReplicationTaskResourceSchema(t)

	if schemaResp.Schema.Description == "" {
		t.Error("expected non-empty schema description")
	}

	attrs := schemaResp.Schema.Attributes
	for _, name := range []string{
		"id", "name", "direction", "transport", "ssh_credentials", "sudo",
		"source_datasets", "target_dataset", "recursive", "exclude",
		"also_include_naming_schema", "auto", "retention_policy", "lifetime_value",
		"lifetime_unit", "readonly", "allow_from_scratch", "compression",
		"speed_limit", "retries", "logging_level", "enabled", "state",
	} {
		if attrs[name] == nil {
			t.Errorf("expected %q attribute", name)
		}
	}

	if schemaResp.Schema.Blocks["schedule"] == nil {
		t.Error("expected 'schedule' block")
	}

	for _, name := range []string{"name", "source_datasets", "target_dataset", "also_include_naming_schema", "retention_policy"} {
		if !attrs[name].IsRequired() {
			t.Errorf("expected %q to be required", name)
		}
	}
	for _, name := range []string{"id", "auto", "state"} {
		if !attrs[name].IsComputed() || attrs[name].IsOptional() {
			t.Errorf("expected %q to be computed-only", name)
		}
	}
	// The transport branch, not the schema, requires the credential, so that a
	// future transport can forbid it without changing this attribute.
	if !attrs["ssh_credentials"].IsOptional() || attrs["ssh_credentials"].IsRequired() {
		t.Error("expected 'ssh_credentials' to be optional")
	}
}

func getReplicationTaskResourceSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	r := NewReplicationTaskResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}
	return *schemaResp
}

// replicationTaskModelParams holds parameters for building test model values.
// Nil list fields become null lists; interface{} scalars allow nil for null.
type replicationTaskModelParams struct {
	ID                      interface{}
	Name                    interface{}
	Direction               interface{}
	Transport               interface{}
	SSHCredentials          interface{}
	Sudo                    interface{}
	SourceDatasets          []string
	TargetDataset           interface{}
	Recursive               interface{}
	Exclude                 []string
	AlsoIncludeNamingSchema []string
	Auto                    interface{}
	RetentionPolicy         interface{}
	LifetimeValue           interface{}
	LifetimeUnit            interface{}
	Readonly                interface{}
	AllowFromScratch        interface{}
	Compression             interface{}
	SpeedLimit              interface{}
	Retries                 interface{}
	LoggingLevel            interface{}
	Enabled                 interface{}
	State                   interface{}
	Schedule                *replicationScheduleParams
}

type replicationScheduleParams struct {
	Minute interface{}
	Hour   interface{}
	Dom    interface{}
	Month  interface{}
	Dow    interface{}
	Begin  interface{}
	End    interface{}
}

var replicationScheduleObjectType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"minute": tftypes.String,
		"hour":   tftypes.String,
		"dom":    tftypes.String,
		"month":  tftypes.String,
		"dow":    tftypes.String,
		"begin":  tftypes.String,
		"end":    tftypes.String,
	},
}

func replicationScheduleValue(p *replicationScheduleParams) tftypes.Value {
	if p == nil {
		return tftypes.NewValue(replicationScheduleObjectType, nil)
	}

	return tftypes.NewValue(replicationScheduleObjectType, map[string]tftypes.Value{
		"minute": tftypes.NewValue(tftypes.String, p.Minute),
		"hour":   tftypes.NewValue(tftypes.String, p.Hour),
		"dom":    tftypes.NewValue(tftypes.String, p.Dom),
		"month":  tftypes.NewValue(tftypes.String, p.Month),
		"dow":    tftypes.NewValue(tftypes.String, p.Dow),
		"begin":  tftypes.NewValue(tftypes.String, p.Begin),
		"end":    tftypes.NewValue(tftypes.String, p.End),
	})
}

var replicationTaskObjectType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"id":                         tftypes.String,
		"name":                       tftypes.String,
		"direction":                  tftypes.String,
		"transport":                  tftypes.String,
		"ssh_credentials":            tftypes.Number,
		"sudo":                       tftypes.Bool,
		"source_datasets":            tftypes.List{ElementType: tftypes.String},
		"target_dataset":             tftypes.String,
		"recursive":                  tftypes.Bool,
		"exclude":                    tftypes.List{ElementType: tftypes.String},
		"also_include_naming_schema": tftypes.List{ElementType: tftypes.String},
		"auto":                       tftypes.Bool,
		"retention_policy":           tftypes.String,
		"lifetime_value":             tftypes.Number,
		"lifetime_unit":              tftypes.String,
		"readonly":                   tftypes.String,
		"allow_from_scratch":         tftypes.Bool,
		"compression":                tftypes.String,
		"speed_limit":                tftypes.Number,
		"retries":                    tftypes.Number,
		"logging_level":              tftypes.String,
		"enabled":                    tftypes.Bool,
		"state":                      tftypes.String,
		"schedule":                   replicationScheduleObjectType,
	},
}

func createReplicationTaskModelValue(p replicationTaskModelParams) tftypes.Value {
	return tftypes.NewValue(replicationTaskObjectType, map[string]tftypes.Value{
		"id":                         tftypes.NewValue(tftypes.String, p.ID),
		"name":                       tftypes.NewValue(tftypes.String, p.Name),
		"direction":                  tftypes.NewValue(tftypes.String, p.Direction),
		"transport":                  tftypes.NewValue(tftypes.String, p.Transport),
		"ssh_credentials":            tftypes.NewValue(tftypes.Number, p.SSHCredentials),
		"sudo":                       tftypes.NewValue(tftypes.Bool, p.Sudo),
		"source_datasets":            stringListValue(p.SourceDatasets),
		"target_dataset":             tftypes.NewValue(tftypes.String, p.TargetDataset),
		"recursive":                  tftypes.NewValue(tftypes.Bool, p.Recursive),
		"exclude":                    stringListValue(p.Exclude),
		"also_include_naming_schema": stringListValue(p.AlsoIncludeNamingSchema),
		"auto":                       tftypes.NewValue(tftypes.Bool, p.Auto),
		"retention_policy":           tftypes.NewValue(tftypes.String, p.RetentionPolicy),
		"lifetime_value":             tftypes.NewValue(tftypes.Number, p.LifetimeValue),
		"lifetime_unit":              tftypes.NewValue(tftypes.String, p.LifetimeUnit),
		"readonly":                   tftypes.NewValue(tftypes.String, p.Readonly),
		"allow_from_scratch":         tftypes.NewValue(tftypes.Bool, p.AllowFromScratch),
		"compression":                tftypes.NewValue(tftypes.String, p.Compression),
		"speed_limit":                tftypes.NewValue(tftypes.Number, p.SpeedLimit),
		"retries":                    tftypes.NewValue(tftypes.Number, p.Retries),
		"logging_level":              tftypes.NewValue(tftypes.String, p.LoggingLevel),
		"enabled":                    tftypes.NewValue(tftypes.Bool, p.Enabled),
		"state":                      tftypes.NewValue(tftypes.String, p.State),
		"schedule":                   replicationScheduleValue(p.Schedule),
	})
}

// fullReplicationTaskParams is a nightly archive-to-backup push, matching
// testReplicationTask.
func fullReplicationTaskParams() replicationTaskModelParams {
	return replicationTaskModelParams{
		Name:                    "archive-to-backup",
		Direction:               "PUSH",
		Transport:               "SSH",
		SSHCredentials:          2,
		Sudo:                    false,
		SourceDatasets:          []string{"tank/archive"},
		TargetDataset:           "tank/backup",
		Recursive:               true,
		Exclude:                 []string{"tank/archive/scratch"},
		AlsoIncludeNamingSchema: []string{"auto-%Y-%m-%d_%H-%M"},
		Auto:                    true,
		RetentionPolicy:         "CUSTOM",
		LifetimeValue:           2,
		LifetimeUnit:            "WEEK",
		Readonly:                "SET",
		AllowFromScratch:        false,
		Compression:             "LZ4",
		SpeedLimit:              1048576,
		Retries:                 5,
		Enabled:                 true,
		State:                   "PENDING",
		Schedule: &replicationScheduleParams{
			Minute: "0", Hour: "3", Dom: "*", Month: "*", Dow: "*",
			Begin: "00:00", End: "23:59",
		},
	}
}

func testReplicationTask() *services.ReplicationTask {
	credential := int64(2)
	lifetimeValue := int64(2)
	lifetimeUnit := "WEEK"
	compression := "LZ4"
	speedLimit := int64(1048576)

	return &services.ReplicationTask{
		ID:                      1,
		Name:                    "archive-to-backup",
		Direction:               "PUSH",
		Transport:               "SSH",
		SSHCredentials:          &credential,
		SourceDatasets:          []string{"tank/archive"},
		TargetDataset:           "tank/backup",
		Recursive:               true,
		Exclude:                 []string{"tank/archive/scratch"},
		AlsoIncludeNamingSchema: []string{"auto-%Y-%m-%d_%H-%M"},
		Auto:                    true,
		Schedule: &services.ReplicationSchedule{
			Minute: "0", Hour: "3", Dom: "*", Month: "*", Dow: "*",
			Begin: "00:00", End: "23:59",
		},
		RetentionPolicy: "CUSTOM",
		LifetimeValue:   &lifetimeValue,
		LifetimeUnit:    &lifetimeUnit,
		Readonly:        "SET",
		Compression:     &compression,
		SpeedLimit:      &speedLimit,
		Retries:         5,
		Enabled:         true,
		State:           "PENDING",
	}
}

func assertReplicationTaskState(t *testing.T, data ReplicationTaskResourceModel) {
	t.Helper()

	if data.ID.ValueString() != "1" {
		t.Errorf("expected ID '1', got %q", data.ID.ValueString())
	}
	if data.Name.ValueString() != "archive-to-backup" {
		t.Errorf("expected name 'archive-to-backup', got %q", data.Name.ValueString())
	}
	if data.SSHCredentials.ValueInt64() != 2 {
		t.Errorf("expected ssh_credentials 2, got %d", data.SSHCredentials.ValueInt64())
	}
	if data.TargetDataset.ValueString() != "tank/backup" {
		t.Errorf("expected target 'tank/backup', got %q", data.TargetDataset.ValueString())
	}
	if !data.Auto.ValueBool() {
		t.Error("expected auto true")
	}
	if data.LifetimeValue.ValueInt64() != 2 || data.LifetimeUnit.ValueString() != "WEEK" {
		t.Errorf("unexpected lifetime %v %v", data.LifetimeValue, data.LifetimeUnit)
	}
	if data.SpeedLimit.ValueInt64() != 1048576 {
		t.Errorf("expected speed_limit 1048576, got %d", data.SpeedLimit.ValueInt64())
	}
	if !data.LoggingLevel.IsNull() {
		t.Error("expected logging_level to be null")
	}
	if data.State.ValueString() != "PENDING" {
		t.Errorf("expected state PENDING, got %q", data.State.ValueString())
	}
	if data.Schedule == nil || data.Schedule.Hour.ValueString() != "3" {
		t.Errorf("unexpected schedule: %+v", data.Schedule)
	}
	if len(data.SourceDatasets.Elements()) != 1 {
		t.Errorf("expected 1 source dataset, got %d", len(data.SourceDatasets.Elements()))
	}
}

func newReplicationTaskResource(mock *services.MockReplicationService) *ReplicationTaskResource {
	return &ReplicationTaskResource{
		BaseResource: BaseResource{services: &services.TrueNASServices{Replication: mock}},
	}
}

func TestReplicationTaskResource_Create_Success(t *testing.T) {
	var capturedOpts services.CreateReplicationTaskOpts

	r := newReplicationTaskResource(&services.MockReplicationService{
		CreateFunc: func(ctx context.Context, opts services.CreateReplicationTaskOpts) (*services.ReplicationTask, error) {
			capturedOpts = opts
			return testReplicationTask(), nil
		},
	})

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(fullReplicationTaskParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if capturedOpts.Name != "archive-to-backup" {
		t.Errorf("expected name 'archive-to-backup', got %q", capturedOpts.Name)
	}
	if capturedOpts.Direction != "PUSH" || capturedOpts.Transport != "SSH" {
		t.Errorf("unexpected direction/transport: %s/%s", capturedOpts.Direction, capturedOpts.Transport)
	}
	if capturedOpts.SSHCredentials == nil || *capturedOpts.SSHCredentials != 2 {
		t.Errorf("expected SSHCredentials 2, got %v", capturedOpts.SSHCredentials)
	}
	if !capturedOpts.Auto {
		t.Error("expected Auto true for a task with a schedule")
	}
	if capturedOpts.Schedule == nil || capturedOpts.Schedule.Hour != "3" {
		t.Errorf("unexpected schedule: %+v", capturedOpts.Schedule)
	}
	if capturedOpts.Compression == nil || *capturedOpts.Compression != "LZ4" {
		t.Errorf("expected compression LZ4, got %v", capturedOpts.Compression)
	}
	if capturedOpts.LoggingLevel != nil {
		t.Errorf("expected nil LoggingLevel, got %v", *capturedOpts.LoggingLevel)
	}
	if len(capturedOpts.Exclude) != 1 || capturedOpts.Exclude[0] != "tank/archive/scratch" {
		t.Errorf("unexpected exclude: %v", capturedOpts.Exclude)
	}

	var data ReplicationTaskResourceModel
	resp.State.Get(context.Background(), &data)
	assertReplicationTaskState(t, data)
}

// TestReplicationTaskResource_Create_WithoutSchedule covers the manual-only
// task: no schedule block means auto is false and no cron object is sent.
func TestReplicationTaskResource_Create_WithoutSchedule(t *testing.T) {
	var capturedOpts services.CreateReplicationTaskOpts

	task := testReplicationTask()
	task.Auto = false
	task.Schedule = nil

	r := newReplicationTaskResource(&services.MockReplicationService{
		CreateFunc: func(ctx context.Context, opts services.CreateReplicationTaskOpts) (*services.ReplicationTask, error) {
			capturedOpts = opts
			return task, nil
		},
	})

	params := fullReplicationTaskParams()
	params.Schedule = nil

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedOpts.Auto {
		t.Error("expected Auto false without a schedule")
	}
	if capturedOpts.Schedule != nil {
		t.Errorf("expected nil schedule, got %+v", capturedOpts.Schedule)
	}

	var data ReplicationTaskResourceModel
	resp.State.Get(context.Background(), &data)
	if data.Schedule != nil {
		t.Errorf("expected nil schedule in state, got %+v", data.Schedule)
	}
}

// TestReplicationTaskResource_Create_ScheduleDefaults covers a schedule block
// that only pins the fields the practitioner cares about: the rest are filled
// with the API's documented defaults rather than sent as empty strings.
func TestReplicationTaskResource_Create_ScheduleDefaults(t *testing.T) {
	var capturedOpts services.CreateReplicationTaskOpts

	r := newReplicationTaskResource(&services.MockReplicationService{
		CreateFunc: func(ctx context.Context, opts services.CreateReplicationTaskOpts) (*services.ReplicationTask, error) {
			capturedOpts = opts
			return testReplicationTask(), nil
		},
	})

	params := fullReplicationTaskParams()
	params.Schedule = &replicationScheduleParams{Minute: "30", Hour: "2"}

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	want := services.ReplicationSchedule{
		Minute: "30", Hour: "2", Dom: "*", Month: "*", Dow: "*",
		Begin: "00:00", End: "23:59",
	}
	if capturedOpts.Schedule == nil || *capturedOpts.Schedule != want {
		t.Errorf("expected schedule %+v, got %+v", want, capturedOpts.Schedule)
	}
}

func TestReplicationTaskResource_Create_NullListsBecomeEmpty(t *testing.T) {
	var capturedOpts services.CreateReplicationTaskOpts

	r := newReplicationTaskResource(&services.MockReplicationService{
		CreateFunc: func(ctx context.Context, opts services.CreateReplicationTaskOpts) (*services.ReplicationTask, error) {
			capturedOpts = opts
			return testReplicationTask(), nil
		},
	})

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(replicationTaskModelParams{
			Name:            "minimal",
			RetentionPolicy: "NONE",
		})},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	for name, got := range map[string][]string{
		"source_datasets":            capturedOpts.SourceDatasets,
		"exclude":                    capturedOpts.Exclude,
		"also_include_naming_schema": capturedOpts.AlsoIncludeNamingSchema,
	} {
		if got == nil || len(got) != 0 {
			t.Errorf("expected empty non-nil slice for %s, got %v", name, got)
		}
	}
	if capturedOpts.SSHCredentials != nil {
		t.Error("expected nil SSHCredentials for a null attribute")
	}
	if capturedOpts.LifetimeValue != nil {
		t.Error("expected nil LifetimeValue for a null attribute")
	}
}

func TestReplicationTaskResource_Create_APIError(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{
		CreateFunc: func(ctx context.Context, opts services.CreateReplicationTaskOpts) (*services.ReplicationTask, error) {
			return nil, errors.New("connection refused")
		},
	})

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(fullReplicationTaskParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestReplicationTaskResource_Create_NilTask(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{})

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(fullReplicationTaskParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for nil task")
	}
}

func TestReplicationTaskResource_Read_Success(t *testing.T) {
	var capturedID int64

	r := newReplicationTaskResource(&services.MockReplicationService{
		GetFunc: func(ctx context.Context, id int64) (*services.ReplicationTask, error) {
			capturedID = id
			return testReplicationTask(), nil
		},
	})

	params := fullReplicationTaskParams()
	params.ID = "1"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 1 {
		t.Errorf("expected ID 1, got %d", capturedID)
	}

	var data ReplicationTaskResourceModel
	resp.State.Get(context.Background(), &data)
	assertReplicationTaskState(t, data)
}

func TestReplicationTaskResource_Read_Deleted(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{
		GetFunc: func(ctx context.Context, id int64) (*services.ReplicationTask, error) { return nil, nil },
	})

	params := fullReplicationTaskParams()
	params.ID = "1"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    createReplicationTaskModelValue(params),
	}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed")
	}
}

func TestReplicationTaskResource_Read_APIError(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{
		GetFunc: func(ctx context.Context, id int64) (*services.ReplicationTask, error) {
			return nil, errors.New("boom")
		},
	})

	params := fullReplicationTaskParams()
	params.ID = "1"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestReplicationTaskResource_Read_InvalidID(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{})

	params := fullReplicationTaskParams()
	params.ID = "not-a-number"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unparseable ID")
	}
}

func TestReplicationTaskResource_Update_Success(t *testing.T) {
	var capturedID int64
	var capturedOpts services.UpdateReplicationTaskOpts

	r := newReplicationTaskResource(&services.MockReplicationService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateReplicationTaskOpts) (*services.ReplicationTask, error) {
			capturedID = id
			capturedOpts = opts
			return testReplicationTask(), nil
		},
	})

	state := fullReplicationTaskParams()
	state.ID = "1"
	plan := fullReplicationTaskParams()
	plan.ID = "1"
	plan.Enabled = false

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(state)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(plan)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 1 {
		t.Errorf("expected ID 1, got %d", capturedID)
	}
	if capturedOpts.Enabled {
		t.Error("expected Enabled false in update opts")
	}
}

func TestReplicationTaskResource_Update_APIError(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateReplicationTaskOpts) (*services.ReplicationTask, error) {
			return nil, errors.New("boom")
		},
	})

	params := fullReplicationTaskParams()
	params.ID = "1"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestReplicationTaskResource_Update_NilTask(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{})

	params := fullReplicationTaskParams()
	params.ID = "1"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for nil task")
	}
}

func TestReplicationTaskResource_Update_InvalidID(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{})

	params := fullReplicationTaskParams()
	params.ID = "nope"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unparseable ID")
	}
}

func TestReplicationTaskResource_Delete_Success(t *testing.T) {
	var capturedID int64

	r := newReplicationTaskResource(&services.MockReplicationService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			capturedID = id
			return nil
		},
	})

	params := fullReplicationTaskParams()
	params.ID = "1"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 1 {
		t.Errorf("expected ID 1, got %d", capturedID)
	}
}

func TestReplicationTaskResource_Delete_APIError(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{
		DeleteFunc: func(ctx context.Context, id int64) error { return errors.New("boom") },
	})

	params := fullReplicationTaskParams()
	params.ID = "1"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestReplicationTaskResource_Delete_InvalidID(t *testing.T) {
	r := newReplicationTaskResource(&services.MockReplicationService{})

	params := fullReplicationTaskParams()
	params.ID = "nope"

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unparseable ID")
	}
}

func TestReplicationTaskResource_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*replicationTaskModelParams)
		wantErr string
	}{
		{
			name:   "valid nightly push",
			mutate: func(p *replicationTaskModelParams) {},
		},
		{
			name:    "ssh transport without credentials",
			mutate:  func(p *replicationTaskModelParams) { p.SSHCredentials = nil },
			wantErr: "Missing SSH Credentials",
		},
		{
			name: "custom retention without lifetime",
			mutate: func(p *replicationTaskModelParams) {
				p.LifetimeValue = nil
				p.LifetimeUnit = nil
			},
			wantErr: "Missing Snapshot Lifetime",
		},
		{
			name: "source retention with lifetime",
			mutate: func(p *replicationTaskModelParams) {
				p.RetentionPolicy = "SOURCE"
			},
			wantErr: "Unexpected Snapshot Lifetime",
		},
		{
			name: "none retention without lifetime",
			mutate: func(p *replicationTaskModelParams) {
				p.RetentionPolicy = "NONE"
				p.LifetimeValue = nil
				p.LifetimeUnit = nil
			},
		},
		{
			name: "exclude without recursive",
			mutate: func(p *replicationTaskModelParams) {
				p.Recursive = false
			},
			wantErr: "Exclusions Require Recursive Replication",
		},
		{
			name: "no exclusions without recursive",
			mutate: func(p *replicationTaskModelParams) {
				p.Recursive = false
				p.Exclude = []string{}
			},
		},
		{
			name: "unset retention policy defers to the schema",
			mutate: func(p *replicationTaskModelParams) {
				p.RetentionPolicy = nil
				p.LifetimeValue = nil
				p.LifetimeUnit = nil
			},
		},
		{
			name: "unset transport defaults to ssh",
			mutate: func(p *replicationTaskModelParams) {
				p.Transport = nil
				p.SSHCredentials = nil
			},
			wantErr: "Missing SSH Credentials",
		},
	}

	schemaResp := getReplicationTaskResourceSchema(t)
	r := NewReplicationTaskResource().(*ReplicationTaskResource)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := fullReplicationTaskParams()
			tt.mutate(&params)

			resp := &resource.ValidateConfigResponse{}
			r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{
				Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(params)},
			}, resp)

			if tt.wantErr == "" {
				if resp.Diagnostics.HasError() {
					t.Fatalf("unexpected errors: %v", resp.Diagnostics)
				}
				return
			}

			if !resp.Diagnostics.HasError() {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			for _, d := range resp.Diagnostics.Errors() {
				if d.Summary() == tt.wantErr {
					return
				}
			}
			t.Fatalf("expected error %q, got %v", tt.wantErr, resp.Diagnostics)
		})
	}
}

// TestReplicationTaskResource_ValidateConfig_UnknownValues covers a config
// whose values are not resolved until apply: no rule may fire on an unknown.
func TestReplicationTaskResource_ValidateConfig_UnknownValues(t *testing.T) {
	schemaResp := getReplicationTaskResourceSchema(t)
	r := NewReplicationTaskResource().(*ReplicationTaskResource)

	value := createReplicationTaskModelValue(fullReplicationTaskParams())
	var attrs map[string]tftypes.Value
	if err := value.As(&attrs); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	attrs["transport"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	attrs["retention_policy"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	attrs["recursive"] = tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)

	resp := &resource.ValidateConfigResponse{}
	r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(value.Type(), attrs)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestReplicationTaskResource_ValidateConfig_InvalidConfig(t *testing.T) {
	schemaResp := getReplicationTaskResourceSchema(t)
	r := NewReplicationTaskResource().(*ReplicationTaskResource)

	// A raw value that does not match the schema: reading it must surface a
	// diagnostic rather than running the rules against a half-built model.
	resp := &resource.ValidateConfigResponse{}
	r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{
		Config: tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{}),
		},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for an unreadable config")
	}
}
