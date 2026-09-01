package resources

import (
	"context"
	"testing"

	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/deevus/terraform-provider-truenas/internal/services"
)

// replicationTestProvider exposes just the replication task resource, so the
// framework's own plan machinery — schema defaults included — can be driven
// from a unit test.
type replicationTestProvider struct {
	services *services.TrueNASServices
}

func (p *replicationTestProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "truenas"
}

func (p *replicationTestProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{}
}

func (p *replicationTestProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	resp.ResourceData = p.services
}

func (p *replicationTestProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewReplicationTaskResource}
}

func (p *replicationTestProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// planReplicationTask runs the framework's PlanResourceChange for the given
// prior state, proposed new state and configuration, returning the planned
// state Terraform would apply.
func planReplicationTask(t *testing.T, prior, proposed, config tftypes.Value) tftypes.Value {
	t.Helper()

	ctx := context.Background()
	server := providerserver.NewProtocol6(&replicationTestProvider{})()

	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %s", err)
	}

	objType := schemaResp.ResourceSchemas["truenas_replication_task"].ValueType()

	dynamic := func(v tftypes.Value) *tfprotov6.DynamicValue {
		t.Helper()
		dv, err := tfprotov6.NewDynamicValue(objType, v)
		if err != nil {
			t.Fatalf("NewDynamicValue: %s", err)
		}
		return &dv
	}

	planResp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "truenas_replication_task",
		PriorState:       dynamic(prior),
		ProposedNewState: dynamic(proposed),
		Config:           dynamic(config),
	})
	if err != nil {
		t.Fatalf("PlanResourceChange: %s", err)
	}
	for _, d := range planResp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("plan diagnostic: %s: %s", d.Summary, d.Detail)
		}
	}

	planned, err := planResp.PlannedState.Unmarshal(objType)
	if err != nil {
		t.Fatalf("unmarshal planned state: %s", err)
	}

	return planned
}

func plannedReplicationSchedule(t *testing.T, planned tftypes.Value) *ReplicationTaskScheduleModel {
	t.Helper()

	schemaResp := getReplicationTaskResourceSchema(t)

	var data ReplicationTaskResourceModel
	plan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: planned}
	if diags := plan.Get(context.Background(), &data); diags.HasError() {
		t.Fatalf("reading planned state: %v", diags)
	}

	return data.Schedule
}

// TestReplicationTaskResource_Plan_ScheduleDefaults covers a schedule block that
// only pins the fields the practitioner cares about: the rest are planned as the
// API's documented defaults rather than left null.
func TestReplicationTaskResource_Plan_ScheduleDefaults(t *testing.T) {
	config := fullReplicationTaskParams()
	config.ID = nil
	config.Auto = nil
	config.State = nil
	config.Schedule = &replicationScheduleParams{Minute: "30", Hour: "2"}

	proposed := config
	proposed.ID = tftypes.UnknownValue
	proposed.Auto = tftypes.UnknownValue
	proposed.State = tftypes.UnknownValue

	planned := planReplicationTask(t,
		tftypes.NewValue(replicationTaskObjectType, nil),
		createReplicationTaskModelValue(proposed),
		createReplicationTaskModelValue(config),
	)

	schedule := plannedReplicationSchedule(t, planned)
	if schedule == nil {
		t.Fatal("expected a planned schedule")
	}

	for name, got := range map[string]string{
		"minute": schedule.Minute.ValueString(),
		"hour":   schedule.Hour.ValueString(),
		"dom":    schedule.Dom.ValueString(),
		"month":  schedule.Month.ValueString(),
		"dow":    schedule.Dow.ValueString(),
		"begin":  schedule.Begin.ValueString(),
		"end":    schedule.End.ValueString(),
	} {
		want := map[string]string{
			"minute": "30", "hour": "2", "dom": "*", "month": "*", "dow": "*",
			"begin": "00:00", "end": "23:59",
		}[name]
		if got != want {
			t.Errorf("expected planned %s %q, got %q", name, want, got)
		}
	}
}

// TestReplicationTaskResource_Plan_ScheduleFieldRemoved covers dropping a cron
// field from an existing task: Terraform proposes the prior 03:00 hour for the
// now-null attribute, and the schema default has to plan it back to `*` so the
// removal reaches the API instead of being silently ignored.
func TestReplicationTaskResource_Plan_ScheduleFieldRemoved(t *testing.T) {
	prior := fullReplicationTaskParams()
	prior.ID = "1"

	config := fullReplicationTaskParams()
	config.ID = nil
	config.Auto = nil
	config.State = nil
	config.Schedule = &replicationScheduleParams{Minute: "0"}

	// Terraform copies the prior value into a config-null optional+computed
	// attribute when building the proposed new state.
	proposed := config
	proposed.ID = "1"
	proposed.Auto = true
	proposed.State = "PENDING"
	proposed.Schedule = &replicationScheduleParams{
		Minute: "0", Hour: "3", Dom: "*", Month: "*", Dow: "*",
		Begin: "00:00", End: "23:59",
	}

	planned := planReplicationTask(t,
		createReplicationTaskModelValue(prior),
		createReplicationTaskModelValue(proposed),
		createReplicationTaskModelValue(config),
	)

	if got := plannedReplicationSchedule(t, planned).Hour.ValueString(); got != "*" {
		t.Fatalf("expected the removed hour to plan back to \"*\", got %q", got)
	}

	var capturedOpts services.CreateReplicationTaskOpts

	r := newReplicationTaskResource(&services.MockReplicationService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.CreateReplicationTaskOpts) (*services.ReplicationTask, error) {
			capturedOpts = opts
			return testReplicationTask(), nil
		},
	})

	schemaResp := getReplicationTaskResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createReplicationTaskModelValue(prior)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: planned},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedOpts.Schedule == nil || capturedOpts.Schedule.Hour != "*" {
		t.Errorf("expected the API to receive hour \"*\", got %+v", capturedOpts.Schedule)
	}
}

// TestReplicationTaskResource_Plan_WithoutSchedule guards the manual task: the
// schedule attributes carry defaults, but an omitted block must stay null so
// auto stays false and no cron object is sent.
func TestReplicationTaskResource_Plan_WithoutSchedule(t *testing.T) {
	config := fullReplicationTaskParams()
	config.ID = nil
	config.Auto = nil
	config.State = nil
	config.Schedule = nil

	proposed := config
	proposed.ID = tftypes.UnknownValue
	proposed.Auto = tftypes.UnknownValue
	proposed.State = tftypes.UnknownValue

	planned := planReplicationTask(t,
		tftypes.NewValue(replicationTaskObjectType, nil),
		createReplicationTaskModelValue(proposed),
		createReplicationTaskModelValue(config),
	)

	if schedule := plannedReplicationSchedule(t, planned); schedule != nil {
		t.Fatalf("expected an omitted schedule block to stay null, got %+v", schedule)
	}
}

// replicationTaskServer returns a configured provider server backed by mock, so
// import and refresh can be driven through the framework's real RPC paths.
func replicationTaskServer(t *testing.T, mock *services.MockReplicationService) tfprotov6.ProviderServer {
	t.Helper()

	ctx := context.Background()
	server := providerserver.NewProtocol6(&replicationTestProvider{
		services: &services.TrueNASServices{Replication: mock},
	})()

	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %s", err)
	}

	providerType := schemaResp.Provider.ValueType()
	config, err := tfprotov6.NewDynamicValue(providerType, tftypes.NewValue(providerType, map[string]tftypes.Value{}))
	if err != nil {
		t.Fatalf("NewDynamicValue: %s", err)
	}

	configureResp, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{Config: &config})
	if err != nil {
		t.Fatalf("ConfigureProvider: %s", err)
	}
	assertNoErrorDiagnostics(t, configureResp.Diagnostics)

	return server
}

func assertNoErrorDiagnostics(t *testing.T, diagnostics []*tfprotov6.Diagnostic) {
	t.Helper()

	for _, d := range diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("unexpected error diagnostic: %s: %s", d.Summary, d.Detail)
		}
	}
}

// importReplicationTask runs ImportResourceState followed by the ReadResource
// the framework performs next, which is where the task actually arrives.
func importReplicationTask(t *testing.T, server tfprotov6.ProviderServer, id string) *tfprotov6.ReadResourceResponse {
	t.Helper()

	ctx := context.Background()

	importResp, err := server.ImportResourceState(ctx, &tfprotov6.ImportResourceStateRequest{
		TypeName: "truenas_replication_task",
		ID:       id,
	})
	if err != nil {
		t.Fatalf("ImportResourceState: %s", err)
	}
	assertNoErrorDiagnostics(t, importResp.Diagnostics)

	if len(importResp.ImportedResources) != 1 {
		t.Fatalf("expected 1 imported resource, got %d", len(importResp.ImportedResources))
	}

	imported := importResp.ImportedResources[0]

	readResp, err := server.ReadResource(ctx, &tfprotov6.ReadResourceRequest{
		TypeName:     "truenas_replication_task",
		CurrentState: imported.State,
		Private:      imported.Private,
	})
	if err != nil {
		t.Fatalf("ReadResource: %s", err)
	}

	return readResp
}

func replicationTaskStateModel(t *testing.T, state *tfprotov6.DynamicValue) ReplicationTaskResourceModel {
	t.Helper()

	schemaResp := getReplicationTaskResourceSchema(t)
	raw, err := state.Unmarshal(replicationTaskObjectType)
	if err != nil {
		t.Fatalf("unmarshal state: %s", err)
	}

	var data ReplicationTaskResourceModel
	if diags := (tfsdk.State{Schema: schemaResp.Schema, Raw: raw}).Get(context.Background(), &data); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}

	return data
}

// TestReplicationTaskResource_Import_UnsupportedMode covers importing a task
// this resource does not manage: the import fails rather than adopting a task
// the next apply would rewrite.
func TestReplicationTaskResource_Import_UnsupportedMode(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*services.ReplicationTask)
		summary string
		value   string
	}{
		{
			name:    "direction",
			mutate:  func(task *services.ReplicationTask) { task.Direction = "PULL" },
			summary: "Unsupported Replication Task Direction",
			value:   "PULL",
		},
		{
			name: "transport",
			mutate: func(task *services.ReplicationTask) {
				task.Transport = "LOCAL"
				task.SSHCredentials = nil
			},
			summary: "Unsupported Replication Task Transport",
			value:   "LOCAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := testReplicationTask()
			tt.mutate(task)

			server := replicationTaskServer(t, &services.MockReplicationService{
				GetFunc: func(ctx context.Context, id int64) (*services.ReplicationTask, error) {
					return task, nil
				},
			})

			readResp := importReplicationTask(t, server, "1")

			var errs []*tfprotov6.Diagnostic
			for _, d := range readResp.Diagnostics {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					errs = append(errs, d)
				}
			}

			if len(errs) != 1 {
				t.Fatalf("expected exactly 1 error diagnostic, got %d: %v", len(errs), readResp.Diagnostics)
			}
			if errs[0].Summary != tt.summary {
				t.Errorf("expected summary %q, got %q", tt.summary, errs[0].Summary)
			}
			if !strings.Contains(errs[0].Detail, tt.value) ||
				!strings.Contains(errs[0].Detail, "truenas_replication_task") {
				t.Errorf("expected the detail to name the resource and %q, got %q", tt.value, errs[0].Detail)
			}
		})
	}
}

// TestReplicationTaskResource_Import_Supported covers the ordinary import, and
// the refresh that follows it: the import marker must not survive into later
// reads, or a task imported successfully would start failing to refresh.
func TestReplicationTaskResource_Import_Supported(t *testing.T) {
	server := replicationTaskServer(t, &services.MockReplicationService{
		GetFunc: func(ctx context.Context, id int64) (*services.ReplicationTask, error) {
			return testReplicationTask(), nil
		},
	})

	readResp := importReplicationTask(t, server, "1")
	assertNoErrorDiagnostics(t, readResp.Diagnostics)

	if len(readResp.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics importing a PUSH/SSH task, got %v", readResp.Diagnostics)
	}

	data := replicationTaskStateModel(t, readResp.NewState)
	if data.ID.ValueString() != "1" || data.Direction.ValueString() != "PUSH" {
		t.Fatalf("unexpected imported state: %s/%s", data.ID.ValueString(), data.Direction.ValueString())
	}

	// The task is flipped out of scope after the import, so a later refresh that
	// still carried the import marker would surface as an error.
	server = replicationTaskServer(t, &services.MockReplicationService{
		GetFunc: func(ctx context.Context, id int64) (*services.ReplicationTask, error) {
			task := testReplicationTask()
			task.Transport = "LOCAL"
			return task, nil
		},
	})

	refreshResp, err := server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{
		TypeName:     "truenas_replication_task",
		CurrentState: readResp.NewState,
		Private:      readResp.Private,
	})
	if err != nil {
		t.Fatalf("ReadResource: %s", err)
	}

	assertNoErrorDiagnostics(t, refreshResp.Diagnostics)

	if len(refreshResp.Diagnostics) != 1 ||
		refreshResp.Diagnostics[0].Severity != tfprotov6.DiagnosticSeverityWarning {
		t.Fatalf("expected a single warning on refresh, got %v", refreshResp.Diagnostics)
	}
	if got := replicationTaskStateModel(t, refreshResp.NewState).Transport.ValueString(); got != "LOCAL" {
		t.Errorf("expected the refreshed task to stay in state as LOCAL, got %q", got)
	}
}
