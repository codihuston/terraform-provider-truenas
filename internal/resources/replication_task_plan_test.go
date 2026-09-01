package resources

import (
	"context"
	"testing"

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
type replicationTestProvider struct{}

func (p *replicationTestProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "truenas"
}

func (p *replicationTestProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{}
}

func (p *replicationTestProvider) Configure(_ context.Context, _ provider.ConfigureRequest, _ *provider.ConfigureResponse) {
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
