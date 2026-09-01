package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestNewSnapshotTaskResource(t *testing.T) {
	r := NewSnapshotTaskResource()
	if r == nil {
		t.Fatal("NewSnapshotTaskResource returned nil")
	}

	if _, ok := r.(*SnapshotTaskResource); !ok {
		t.Fatalf("expected *SnapshotTaskResource, got %T", r)
	}

	// Verify interface implementations
	_ = resource.Resource(r)
	_ = resource.ResourceWithConfigure(r.(*SnapshotTaskResource))
	_ = resource.ResourceWithImportState(r.(*SnapshotTaskResource))
}

func TestSnapshotTaskResource_Metadata(t *testing.T) {
	r := NewSnapshotTaskResource()

	req := resource.MetadataRequest{ProviderTypeName: "truenas"}
	resp := &resource.MetadataResponse{}

	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "truenas_snapshot_task" {
		t.Errorf("expected TypeName 'truenas_snapshot_task', got %q", resp.TypeName)
	}
}

func TestSnapshotTaskResource_Configure_Success(t *testing.T) {
	r := NewSnapshotTaskResource().(*SnapshotTaskResource)

	req := resource.ConfigureRequest{ProviderData: &services.TrueNASServices{}}
	resp := &resource.ConfigureResponse{}

	r.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestSnapshotTaskResource_Configure_NilProviderData(t *testing.T) {
	r := NewSnapshotTaskResource().(*SnapshotTaskResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestSnapshotTaskResource_Configure_WrongType(t *testing.T) {
	r := NewSnapshotTaskResource().(*SnapshotTaskResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "not a client"}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for wrong ProviderData type")
	}
}

func TestSnapshotTaskResource_Schema(t *testing.T) {
	schemaResp := getSnapshotTaskResourceSchema(t)

	if schemaResp.Schema.Description == "" {
		t.Error("expected non-empty schema description")
	}

	attrs := schemaResp.Schema.Attributes
	for _, name := range []string{
		"id", "dataset", "recursive", "exclude", "lifetime_value", "lifetime_unit",
		"naming_schema", "allow_empty", "enabled", "schedule", "vmware_sync",
	} {
		if attrs[name] == nil {
			t.Errorf("expected %q attribute", name)
		}
	}

	if !attrs["dataset"].IsRequired() {
		t.Error("expected 'dataset' to be required")
	}
	if !attrs["id"].IsComputed() {
		t.Error("expected 'id' to be computed")
	}
	// vmware_sync is derived from the VMware-snapshot configuration, so making
	// it settable would guarantee an "inconsistent result after apply" error.
	if !attrs["vmware_sync"].IsComputed() || attrs["vmware_sync"].IsOptional() {
		t.Error("expected 'vmware_sync' to be computed-only")
	}
	if !attrs["schedule"].IsOptional() || !attrs["schedule"].IsComputed() {
		t.Error("expected 'schedule' to be optional and computed")
	}
}

func getSnapshotTaskResourceSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	r := NewSnapshotTaskResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}
	return *schemaResp
}

// snapshotTaskModelParams holds parameters for creating test model values.
// A nil Exclude becomes a null list; interface{} scalars allow nil for null.
type snapshotTaskModelParams struct {
	ID            interface{}
	Dataset       interface{}
	Recursive     interface{}
	Exclude       []string
	LifetimeValue interface{}
	LifetimeUnit  interface{}
	NamingSchema  interface{}
	AllowEmpty    interface{}
	Enabled       interface{}
	Schedule      *snapshotTaskScheduleParams
	VMwareSync    interface{}
}

type snapshotTaskScheduleParams struct {
	Minute interface{}
	Hour   interface{}
	Dom    interface{}
	Month  interface{}
	Dow    interface{}
	Begin  interface{}
	End    interface{}
}

var snapshotTaskScheduleTFType = tftypes.Object{
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

func snapshotTaskScheduleValue(p *snapshotTaskScheduleParams) tftypes.Value {
	if p == nil {
		return tftypes.NewValue(snapshotTaskScheduleTFType, nil)
	}

	return tftypes.NewValue(snapshotTaskScheduleTFType, map[string]tftypes.Value{
		"minute": tftypes.NewValue(tftypes.String, p.Minute),
		"hour":   tftypes.NewValue(tftypes.String, p.Hour),
		"dom":    tftypes.NewValue(tftypes.String, p.Dom),
		"month":  tftypes.NewValue(tftypes.String, p.Month),
		"dow":    tftypes.NewValue(tftypes.String, p.Dow),
		"begin":  tftypes.NewValue(tftypes.String, p.Begin),
		"end":    tftypes.NewValue(tftypes.String, p.End),
	})
}

func createSnapshotTaskModelValue(p snapshotTaskModelParams) tftypes.Value {
	objectType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":             tftypes.String,
			"dataset":        tftypes.String,
			"recursive":      tftypes.Bool,
			"exclude":        tftypes.List{ElementType: tftypes.String},
			"lifetime_value": tftypes.Number,
			"lifetime_unit":  tftypes.String,
			"naming_schema":  tftypes.String,
			"allow_empty":    tftypes.Bool,
			"enabled":        tftypes.Bool,
			"schedule":       snapshotTaskScheduleTFType,
			"vmware_sync":    tftypes.Bool,
		},
	}

	return tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, p.ID),
		"dataset":        tftypes.NewValue(tftypes.String, p.Dataset),
		"recursive":      tftypes.NewValue(tftypes.Bool, p.Recursive),
		"exclude":        stringListValue(p.Exclude),
		"lifetime_value": tftypes.NewValue(tftypes.Number, p.LifetimeValue),
		"lifetime_unit":  tftypes.NewValue(tftypes.String, p.LifetimeUnit),
		"naming_schema":  tftypes.NewValue(tftypes.String, p.NamingSchema),
		"allow_empty":    tftypes.NewValue(tftypes.Bool, p.AllowEmpty),
		"enabled":        tftypes.NewValue(tftypes.Bool, p.Enabled),
		"schedule":       snapshotTaskScheduleValue(p.Schedule),
		"vmware_sync":    tftypes.NewValue(tftypes.Bool, p.VMwareSync),
	})
}

// snapshotTaskValueWithUnknownExclude builds an otherwise valid value whose
// exclude list contains an unknown element.
func snapshotTaskValueWithUnknownExclude() tftypes.Value {
	params := fullSnapshotTaskParams()
	params.ID = "3"

	value := createSnapshotTaskModelValue(params)

	var attrs map[string]tftypes.Value
	if err := value.As(&attrs); err != nil {
		panic(err)
	}
	attrs["exclude"] = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
		tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})

	return tftypes.NewValue(value.Type(), attrs)
}

// testSnapshotTask returns a standard test task for use in tests.
func testSnapshotTask() *services.SnapshotTask {
	return &services.SnapshotTask{
		ID:            3,
		Dataset:       "tank/data",
		Recursive:     true,
		Exclude:       []string{"tank/data/scratch"},
		LifetimeValue: 4,
		LifetimeUnit:  "WEEK",
		NamingSchema:  "nightly-%Y-%m-%d_%H-%M",
		AllowEmpty:    true,
		Enabled:       true,
		Schedule: services.SnapshotSchedule{
			Minute: "0",
			Hour:   "2",
			Dom:    "*",
			Month:  "*",
			Dow:    "*",
			Begin:  "00:00",
			End:    "23:59",
		},
		VMwareSync: false,
	}
}

// fullSnapshotTaskParams matches testSnapshotTask, for use as plan/state input.
func fullSnapshotTaskParams() snapshotTaskModelParams {
	return snapshotTaskModelParams{
		Dataset:       "tank/data",
		Recursive:     true,
		Exclude:       []string{"tank/data/scratch"},
		LifetimeValue: 4,
		LifetimeUnit:  "WEEK",
		NamingSchema:  "nightly-%Y-%m-%d_%H-%M",
		AllowEmpty:    true,
		Enabled:       true,
		Schedule: &snapshotTaskScheduleParams{
			Minute: "0",
			Hour:   "2",
			Dom:    "*",
			Month:  "*",
			Dow:    "*",
			Begin:  "00:00",
			End:    "23:59",
		},
		VMwareSync: false,
	}
}

func assertSnapshotTaskState(t *testing.T, data SnapshotTaskResourceModel) {
	t.Helper()

	if data.ID.ValueString() != "3" {
		t.Errorf("expected ID '3', got %q", data.ID.ValueString())
	}
	if data.Dataset.ValueString() != "tank/data" {
		t.Errorf("expected dataset 'tank/data', got %q", data.Dataset.ValueString())
	}
	if !data.Recursive.ValueBool() {
		t.Error("expected recursive true")
	}
	if data.LifetimeValue.ValueInt64() != 4 {
		t.Errorf("expected lifetime_value 4, got %d", data.LifetimeValue.ValueInt64())
	}
	if data.LifetimeUnit.ValueString() != "WEEK" {
		t.Errorf("expected lifetime_unit 'WEEK', got %q", data.LifetimeUnit.ValueString())
	}
	if data.NamingSchema.ValueString() != "nightly-%Y-%m-%d_%H-%M" {
		t.Errorf("unexpected naming_schema: %q", data.NamingSchema.ValueString())
	}
	if !data.AllowEmpty.ValueBool() {
		t.Error("expected allow_empty true")
	}
	if !data.Enabled.ValueBool() {
		t.Error("expected enabled true")
	}
	if data.VMwareSync.ValueBool() {
		t.Error("expected vmware_sync false")
	}
	if len(data.Exclude.Elements()) != 1 {
		t.Errorf("expected 1 exclude entry, got %d", len(data.Exclude.Elements()))
	}

	schedule := data.Schedule.Attributes()
	for name, want := range map[string]string{
		"minute": "0",
		"hour":   "2",
		"dom":    "*",
		"month":  "*",
		"dow":    "*",
		"begin":  "00:00",
		"end":    "23:59",
	} {
		got := schedule[name].(types.String).ValueString()
		if got != want {
			t.Errorf("expected schedule.%s %q, got %q", name, want, got)
		}
	}
}

func newSnapshotTaskResource(mock *services.MockSnapshotTaskService) *SnapshotTaskResource {
	return &SnapshotTaskResource{
		BaseResource: BaseResource{services: &services.TrueNASServices{SnapshotTask: mock}},
	}
}

func TestSnapshotTaskResource_Create_Success(t *testing.T) {
	var capturedOpts services.CreateSnapshotTaskOpts

	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		CreateFunc: func(ctx context.Context, opts services.CreateSnapshotTaskOpts) (*services.SnapshotTask, error) {
			capturedOpts = opts
			return testSnapshotTask(), nil
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createSnapshotTaskModelValue(fullSnapshotTaskParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if capturedOpts.Dataset != "tank/data" {
		t.Errorf("expected dataset 'tank/data', got %q", capturedOpts.Dataset)
	}
	if !capturedOpts.Recursive {
		t.Error("expected Recursive true")
	}
	if capturedOpts.LifetimeValue != 4 || capturedOpts.LifetimeUnit != "WEEK" {
		t.Errorf("unexpected lifetime: %d %s", capturedOpts.LifetimeValue, capturedOpts.LifetimeUnit)
	}
	if capturedOpts.NamingSchema != "nightly-%Y-%m-%d_%H-%M" {
		t.Errorf("unexpected naming schema: %q", capturedOpts.NamingSchema)
	}
	if len(capturedOpts.Exclude) != 1 || capturedOpts.Exclude[0] != "tank/data/scratch" {
		t.Errorf("unexpected exclude: %v", capturedOpts.Exclude)
	}
	if capturedOpts.Schedule.Minute != "0" || capturedOpts.Schedule.Hour != "2" {
		t.Errorf("unexpected schedule: %+v", capturedOpts.Schedule)
	}
	if capturedOpts.Schedule.Begin != "00:00" || capturedOpts.Schedule.End != "23:59" {
		t.Errorf("unexpected schedule window: %+v", capturedOpts.Schedule)
	}

	var data SnapshotTaskResourceModel
	resp.State.Get(context.Background(), &data)
	assertSnapshotTaskState(t, data)
}

// A null schedule and a null exclude reach the API as the documented defaults
// rather than as zero values the server would reject.
func TestSnapshotTaskResource_Create_NullScheduleUsesDefaults(t *testing.T) {
	var capturedOpts services.CreateSnapshotTaskOpts

	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		CreateFunc: func(ctx context.Context, opts services.CreateSnapshotTaskOpts) (*services.SnapshotTask, error) {
			capturedOpts = opts
			return testSnapshotTask(), nil
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createSnapshotTaskModelValue(snapshotTaskModelParams{
			Dataset:       "tank/data",
			LifetimeValue: 2,
		})},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if capturedOpts.Schedule != snapshotTaskScheduleDefaults {
		t.Errorf("expected API schedule defaults, got %+v", capturedOpts.Schedule)
	}
	if capturedOpts.Exclude == nil || len(capturedOpts.Exclude) != 0 {
		t.Errorf("expected empty (non-nil) exclude, got %v", capturedOpts.Exclude)
	}
}

func TestSnapshotTaskResource_Create_APIError(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		CreateFunc: func(ctx context.Context, opts services.CreateSnapshotTaskOpts) (*services.SnapshotTask, error) {
			return nil, errors.New("connection refused")
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createSnapshotTaskModelValue(fullSnapshotTaskParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to not be set when API returns error")
	}
}

func TestSnapshotTaskResource_Create_NilTask(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		CreateFunc: func(ctx context.Context, opts services.CreateSnapshotTaskOpts) (*services.SnapshotTask, error) {
			return nil, nil
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createSnapshotTaskModelValue(fullSnapshotTaskParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error when task is nil")
	}
}

// An unknown list element cannot be converted to a string, so the guard after
// buildSnapshotTaskOpts must surface the diagnostic instead of calling the API.
func TestSnapshotTaskResource_Create_UnknownListElement(t *testing.T) {
	called := false
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		CreateFunc: func(ctx context.Context, opts services.CreateSnapshotTaskOpts) (*services.SnapshotTask, error) {
			called = true
			return testSnapshotTask(), nil
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: snapshotTaskValueWithUnknownExclude()},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unknown list element")
	}
	if called {
		t.Error("expected Create not to reach the API when conversion fails")
	}
}

func TestSnapshotTaskResource_Update_UnknownListElement(t *testing.T) {
	called := false
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateSnapshotTaskOpts) (*services.SnapshotTask, error) {
			called = true
			return testSnapshotTask(), nil
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	stateParams := fullSnapshotTaskParams()
	stateParams.ID = "3"

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createSnapshotTaskModelValue(stateParams)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: snapshotTaskValueWithUnknownExclude()},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unknown list element")
	}
	if called {
		t.Error("expected Update not to reach the API when conversion fails")
	}
}

func TestSnapshotTaskResource_Create_InvalidPlan(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{})

	schemaResp := getSnapshotTaskResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	// A plan whose type does not match the schema fails to decode.
	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{}),
		},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable plan")
	}
}

func TestSnapshotTaskResource_Read_Success(t *testing.T) {
	var capturedID int64

	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		GetFunc: func(ctx context.Context, id int64) (*services.SnapshotTask, error) {
			capturedID = id
			return testSnapshotTask(), nil
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "3"
	stateValue := createSnapshotTaskModelValue(params)

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 3 {
		t.Errorf("expected id 3, got %d", capturedID)
	}

	var data SnapshotTaskResourceModel
	resp.State.Get(context.Background(), &data)
	assertSnapshotTaskState(t, data)
}

func TestSnapshotTaskResource_Read_NotFound(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		GetFunc: func(ctx context.Context, id int64) (*services.SnapshotTask, error) { return nil, nil },
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "3"
	stateValue := createSnapshotTaskModelValue(params)

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed when task no longer exists")
	}
}

func TestSnapshotTaskResource_Read_APIError(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		GetFunc: func(ctx context.Context, id int64) (*services.SnapshotTask, error) {
			return nil, errors.New("connection refused")
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "3"
	stateValue := createSnapshotTaskModelValue(params)

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSnapshotTaskResource_Read_InvalidID(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "not-a-number"
	stateValue := createSnapshotTaskModelValue(params)

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestSnapshotTaskResource_Read_InvalidState(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{})

	schemaResp := getSnapshotTaskResourceSchema(t)
	badValue := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: badValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: badValue},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable state")
	}
}

func TestSnapshotTaskResource_Update_Success(t *testing.T) {
	var capturedID int64
	var capturedOpts services.UpdateSnapshotTaskOpts

	updated := testSnapshotTask()
	updated.Enabled = false
	updated.LifetimeValue = 10
	updated.LifetimeUnit = "DAY"

	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateSnapshotTaskOpts) (*services.SnapshotTask, error) {
			capturedID = id
			capturedOpts = opts
			return updated, nil
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)

	stateParams := fullSnapshotTaskParams()
	stateParams.ID = "3"

	planParams := fullSnapshotTaskParams()
	planParams.ID = "3"
	planParams.Enabled = false
	planParams.LifetimeValue = 10
	planParams.LifetimeUnit = "DAY"

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createSnapshotTaskModelValue(stateParams)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: createSnapshotTaskModelValue(planParams)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 3 {
		t.Errorf("expected id 3, got %d", capturedID)
	}
	if capturedOpts.Enabled {
		t.Error("expected Enabled false in update opts")
	}
	if capturedOpts.LifetimeValue != 10 || capturedOpts.LifetimeUnit != "DAY" {
		t.Errorf("unexpected lifetime in update opts: %d %s", capturedOpts.LifetimeValue, capturedOpts.LifetimeUnit)
	}

	var data SnapshotTaskResourceModel
	resp.State.Get(context.Background(), &data)
	if data.Enabled.ValueBool() {
		t.Error("expected enabled false in state")
	}
	if data.LifetimeValue.ValueInt64() != 10 {
		t.Errorf("expected lifetime_value 10 in state, got %d", data.LifetimeValue.ValueInt64())
	}
}

func TestSnapshotTaskResource_Update_APIError(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateSnapshotTaskOpts) (*services.SnapshotTask, error) {
			return nil, errors.New("connection refused")
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "3"
	value := createSnapshotTaskModelValue(params)

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSnapshotTaskResource_Update_NilTask(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateSnapshotTaskOpts) (*services.SnapshotTask, error) {
			return nil, nil
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "3"
	value := createSnapshotTaskModelValue(params)

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error when task is nil")
	}
}

func TestSnapshotTaskResource_Update_InvalidID(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "not-a-number"
	value := createSnapshotTaskModelValue(params)

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestSnapshotTaskResource_Update_InvalidState(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{})

	schemaResp := getSnapshotTaskResourceSchema(t)
	badValue := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: badValue},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: badValue},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable state")
	}
}

func TestSnapshotTaskResource_Delete_Success(t *testing.T) {
	var capturedID int64
	called := false

	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			called = true
			capturedID = id
			return nil
		},
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "3"
	value := createSnapshotTaskModelValue(params)

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: value}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !called {
		t.Error("expected Delete to be called")
	}
	if capturedID != 3 {
		t.Errorf("expected id 3, got %d", capturedID)
	}
}

func TestSnapshotTaskResource_Delete_APIError(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{
		DeleteFunc: func(ctx context.Context, id int64) error { return errors.New("connection refused") },
	})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "3"
	value := createSnapshotTaskModelValue(params)

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: value}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSnapshotTaskResource_Delete_InvalidID(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{})

	schemaResp := getSnapshotTaskResourceSchema(t)
	params := fullSnapshotTaskParams()
	params.ID = "not-a-number"
	value := createSnapshotTaskModelValue(params)

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: value}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestSnapshotTaskResource_Delete_InvalidState(t *testing.T) {
	r := newSnapshotTaskResource(&services.MockSnapshotTaskService{})

	schemaResp := getSnapshotTaskResourceSchema(t)
	badValue := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: badValue}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: badValue},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable state")
	}
}

func TestSnapshotTaskScheduleFromObject(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	if got := snapshotTaskScheduleFromObject(ctx, types.ObjectNull(snapshotTaskScheduleAttrTypes), &diags); got != snapshotTaskScheduleDefaults {
		t.Errorf("expected defaults for null object, got %+v", got)
	}
	if got := snapshotTaskScheduleFromObject(ctx, types.ObjectUnknown(snapshotTaskScheduleAttrTypes), &diags); got != snapshotTaskScheduleDefaults {
		t.Errorf("expected defaults for unknown object, got %+v", got)
	}
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	schedule := services.SnapshotSchedule{
		Minute: "15", Hour: "*/2", Dom: "1", Month: "6", Dow: "3", Begin: "08:00", End: "17:00",
	}
	if got := snapshotTaskScheduleFromObject(ctx, snapshotTaskScheduleObject(schedule), &diags); got != schedule {
		t.Errorf("expected round trip of %+v, got %+v", schedule, got)
	}
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

// An object whose attributes do not match the schedule type cannot be decoded,
// so the defaults are returned alongside the diagnostic.
func TestSnapshotTaskScheduleFromObject_ConversionError(t *testing.T) {
	var diags diag.Diagnostics

	mismatched := types.ObjectValueMust(
		map[string]attr.Type{"unexpected": types.StringType},
		map[string]attr.Value{"unexpected": types.StringValue("value")},
	)

	got := snapshotTaskScheduleFromObject(context.Background(), mismatched, &diags)

	if !diags.HasError() {
		t.Fatal("expected diagnostics for mismatched object type")
	}
	if got != snapshotTaskScheduleDefaults {
		t.Errorf("expected defaults on conversion failure, got %+v", got)
	}
}

func TestSnapshotTaskStringsFromList(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	if got := snapshotTaskStringsFromList(ctx, types.ListNull(types.StringType), &diags); len(got) != 0 || got == nil {
		t.Errorf("expected empty non-nil slice for null list, got %v", got)
	}
	if got := snapshotTaskStringsFromList(ctx, types.ListUnknown(types.StringType), &diags); len(got) != 0 || got == nil {
		t.Errorf("expected empty non-nil slice for unknown list, got %v", got)
	}

	list, _ := types.ListValueFrom(ctx, types.StringType, []string{"tank/a", "tank/b"})
	got := snapshotTaskStringsFromList(ctx, list, &diags)
	if len(got) != 2 || got[0] != "tank/a" || got[1] != "tank/b" {
		t.Errorf("unexpected slice: %v", got)
	}
	if diags.HasError() {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
}

func TestSnapshotTaskListFromStrings(t *testing.T) {
	list := snapshotTaskListFromStrings([]string{"tank/a", "tank/b"})
	if len(list.Elements()) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(list.Elements()))
	}
	if got := list.Elements()[0].(types.String).ValueString(); got != "tank/a" {
		t.Errorf("expected first element 'tank/a', got %q", got)
	}

	empty := snapshotTaskListFromStrings([]string{})
	if empty.IsNull() || len(empty.Elements()) != 0 {
		t.Errorf("expected empty list, got %v", empty)
	}
}
