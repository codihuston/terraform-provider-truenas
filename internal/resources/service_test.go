package resources

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestNewServiceResource(t *testing.T) {
	r := NewServiceResource()
	if r == nil {
		t.Fatal("NewServiceResource returned nil")
	}

	if _, ok := r.(*ServiceResource); !ok {
		t.Fatalf("expected *ServiceResource, got %T", r)
	}

	// Verify interface implementations
	_ = resource.Resource(r)
	_ = resource.ResourceWithConfigure(r.(*ServiceResource))
	_ = resource.ResourceWithImportState(r.(*ServiceResource))
}

func TestServiceResource_Metadata(t *testing.T) {
	r := NewServiceResource()

	req := resource.MetadataRequest{ProviderTypeName: "truenas"}
	resp := &resource.MetadataResponse{}

	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "truenas_service" {
		t.Errorf("expected TypeName 'truenas_service', got %q", resp.TypeName)
	}
}

func TestServiceResource_Configure_Success(t *testing.T) {
	r := NewServiceResource().(*ServiceResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &services.TrueNASServices{}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestServiceResource_Configure_NilProviderData(t *testing.T) {
	r := NewServiceResource().(*ServiceResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestServiceResource_Configure_WrongType(t *testing.T) {
	r := NewServiceResource().(*ServiceResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "not a client"}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for wrong ProviderData type")
	}
}

func TestServiceResource_Schema(t *testing.T) {
	schemaResp := getServiceResourceSchema(t)

	if schemaResp.Schema.Description == "" {
		t.Error("expected non-empty schema description")
	}
	// Destroying the resource stops and disables the service, which the schema
	// description has to spell out.
	if !strings.Contains(schemaResp.Schema.Description, "stops the service and disables it at boot") {
		t.Error("expected schema description to document the destroy behaviour")
	}

	attrs := schemaResp.Schema.Attributes
	for _, name := range []string{"id", "name", "enable", "running"} {
		if attrs[name] == nil {
			t.Errorf("expected %q attribute", name)
		}
	}

	if !attrs["name"].IsRequired() {
		t.Error("expected 'name' to be required")
	}
	if !attrs["id"].IsComputed() || attrs["id"].IsOptional() {
		t.Error("expected 'id' to be computed-only")
	}
	for _, name := range []string{"enable", "running"} {
		if !attrs[name].IsOptional() || !attrs[name].IsComputed() {
			t.Errorf("expected %q to be optional and computed", name)
		}
	}
}

func getServiceResourceSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	r := NewServiceResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}
	return *schemaResp
}

// serviceModelParams holds parameters for creating test model values.
// interface{} scalars allow nil for null.
type serviceModelParams struct {
	ID      interface{}
	Name    interface{}
	Enable  interface{}
	Running interface{}
}

func createServiceModelValue(p serviceModelParams) tftypes.Value {
	objectType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":      tftypes.String,
			"name":    tftypes.String,
			"enable":  tftypes.Bool,
			"running": tftypes.Bool,
		},
	}

	return tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":      tftypes.NewValue(tftypes.String, p.ID),
		"name":    tftypes.NewValue(tftypes.String, p.Name),
		"enable":  tftypes.NewValue(tftypes.Bool, p.Enable),
		"running": tftypes.NewValue(tftypes.Bool, p.Running),
	})
}

// enabledServiceParams describes an nfs service that should be enabled and running.
func enabledServiceParams() serviceModelParams {
	return serviceModelParams{ID: "nfs", Name: "nfs", Enable: true, Running: true}
}

// stoppedService is the state of nfs before Terraform touches it.
func stoppedService() *services.SystemService {
	return &services.SystemService{ID: 9, Name: "nfs", Enable: false, State: "STOPPED"}
}

// runningService is the state of nfs once enabled and started.
func runningService() *services.SystemService {
	return &services.SystemService{ID: 9, Name: "nfs", Enable: true, State: services.ServiceStateRunning}
}

func newServiceResource(mock *services.MockSystemServices) *ServiceResource {
	return &ServiceResource{
		BaseResource: BaseResource{services: &services.TrueNASServices{Service: mock}},
	}
}

// serviceRecorder is a MockSystemServices that walks a scripted sequence of Get
// results while recording the control calls made against it.
type serviceRecorder struct {
	gets    []*services.SystemService
	getCall int

	enableCalls []bool
	started     int
	stopped     int
}

func (rec *serviceRecorder) mock() *services.MockSystemServices {
	return &services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			svc := rec.gets[min(rec.getCall, len(rec.gets)-1)]
			rec.getCall++
			return svc, nil
		},
		SetEnableFunc: func(_ context.Context, _ string, enable bool) error {
			rec.enableCalls = append(rec.enableCalls, enable)
			return nil
		},
		StartFunc: func(context.Context, string) error { rec.started++; return nil },
		StopFunc:  func(context.Context, string) error { rec.stopped++; return nil },
	}
}

func TestServiceResource_Create_EnablesAndStarts(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{stoppedService(), runningService()}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if len(rec.enableCalls) != 1 || !rec.enableCalls[0] {
		t.Errorf("expected one SetEnable(true) call, got %v", rec.enableCalls)
	}
	if rec.started != 1 {
		t.Errorf("expected 1 start, got %d", rec.started)
	}
	if rec.stopped != 0 {
		t.Errorf("expected no stops, got %d", rec.stopped)
	}

	var data ServiceResourceModel
	resp.State.Get(context.Background(), &data)
	if data.ID.ValueString() != "nfs" {
		t.Errorf("expected ID 'nfs', got %q", data.ID.ValueString())
	}
	if !data.Enable.ValueBool() || !data.Running.ValueBool() {
		t.Errorf("expected enable and running true, got %v/%v", data.Enable, data.Running)
	}
}

// A service already in the desired state must not be touched: reconciling is
// conditional, so a no-op plan issues no control calls.
func TestServiceResource_Create_AlreadyInDesiredState(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{runningService()}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if len(rec.enableCalls) != 0 || rec.started != 0 || rec.stopped != 0 {
		t.Errorf("expected no control calls, got enable=%v start=%d stop=%d",
			rec.enableCalls, rec.started, rec.stopped)
	}
}

func TestServiceResource_Create_DisablesAndStops(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{runningService(), stoppedService()}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(
			serviceModelParams{ID: "nfs", Name: "nfs", Enable: false, Running: false})},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if len(rec.enableCalls) != 1 || rec.enableCalls[0] {
		t.Errorf("expected one SetEnable(false) call, got %v", rec.enableCalls)
	}
	if rec.stopped != 1 {
		t.Errorf("expected 1 stop, got %d", rec.stopped)
	}

	var data ServiceResourceModel
	resp.State.Get(context.Background(), &data)
	if data.Enable.ValueBool() || data.Running.ValueBool() {
		t.Errorf("expected enable and running false, got %v/%v", data.Enable, data.Running)
	}
}

// An unknown service name must fail with the names the appliance does offer.
func TestServiceResource_Create_UnknownService(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		ListFunc: func(context.Context) ([]services.SystemService, error) {
			return []services.SystemService{{Name: "ssh"}, {Name: "nfs"}}, nil
		},
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(
			serviceModelParams{ID: "bogus", Name: "bogus", Enable: true, Running: true})},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unknown service")
	}
	detail := resp.Diagnostics.Errors()[0].Detail()
	if !strings.Contains(detail, "Known services: nfs, ssh.") {
		t.Errorf("expected sorted known-service list in detail, got %q", detail)
	}
}

// The known-service list is a courtesy: when it cannot be fetched the original
// error still has to surface, without a dangling sentence.
func TestServiceResource_Create_UnknownService_ListFails(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		ListFunc: func(context.Context) ([]services.SystemService, error) {
			return nil, errors.New("boom")
		},
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(
			serviceModelParams{ID: "bogus", Name: "bogus", Enable: true, Running: true})},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unknown service")
	}
	if detail := resp.Diagnostics.Errors()[0].Detail(); strings.Contains(detail, "Known services") {
		t.Errorf("expected no known-service list when List fails, got %q", detail)
	}
}

func TestServiceResource_Create_GetError(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			return nil, errors.New("boom")
		},
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestServiceResource_Create_SetEnableError(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			return stoppedService(), nil
		},
		SetEnableFunc: func(context.Context, string, bool) error { return errors.New("boom") },
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
	if summary := resp.Diagnostics.Errors()[0].Summary(); summary != "Unable to Update Service" {
		t.Errorf("unexpected summary %q", summary)
	}
}

func TestServiceResource_Create_StartError(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			return &services.SystemService{ID: 9, Name: "nfs", Enable: true, State: "STOPPED"}, nil
		},
		StartFunc: func(context.Context, string) error { return errors.New("boom") },
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
	if detail := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, `start service "nfs"`) {
		t.Errorf("expected the failed operation to be named, got %q", detail)
	}
}

func TestServiceResource_Create_StopError(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			return &services.SystemService{ID: 9, Name: "nfs", Enable: false, State: services.ServiceStateRunning}, nil
		},
		StopFunc: func(context.Context, string) error { return errors.New("boom") },
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(
			serviceModelParams{ID: "nfs", Name: "nfs", Enable: false, Running: false})},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
	if detail := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, `stop service "nfs"`) {
		t.Errorf("expected the failed operation to be named, got %q", detail)
	}
}

// The refresh after a successful control call has its own failure path.
func TestServiceResource_Create_RefreshError(t *testing.T) {
	calls := 0
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			calls++
			if calls == 1 {
				return stoppedService(), nil
			}
			return nil, errors.New("boom")
		},
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestServiceResource_Create_RefreshNotFound(t *testing.T) {
	calls := 0
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			calls++
			if calls == 1 {
				return stoppedService(), nil
			}
			return nil, nil
		},
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
	if summary := resp.Diagnostics.Errors()[0].Summary(); summary != "Service Not Found" {
		t.Errorf("unexpected summary %q", summary)
	}
}

func TestServiceResource_Create_ServiceDoesNotStayRunning(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{
		stoppedService(),
		{ID: 9, Name: "nfs", Enable: true, State: "STOPPED"},
	}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	assertServiceDiagnostic(t, resp.Diagnostics, "Service Not Running", "STOPPED")
	if rec.started != 1 {
		t.Errorf("expected 1 start, got %d", rec.started)
	}

	// The enable did land, so it has to reach state even though the apply failed.
	data := serviceStateModel(t, resp.State)
	if !data.Enable.ValueBool() || data.Running.ValueBool() {
		t.Errorf("expected state enable=true running=false, got %v/%v", data.Enable, data.Running)
	}
	if data.ID.ValueString() != "nfs" {
		t.Errorf("expected state to be populated with id 'nfs', got %v", data.ID)
	}
}

func TestServiceResource_Create_ServiceDiesWithoutControlCall(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{
		runningService(),
		{ID: 9, Name: "nfs", Enable: true, State: "STOPPED"},
	}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	assertServiceDiagnostic(t, resp.Diagnostics, "Service Not Running", "STOPPED")
	// Nothing was out of line on the first read, so no control call was issued
	// and the diagnostic must not claim one was.
	if rec.started != 0 || rec.stopped != 0 || len(rec.enableCalls) != 0 {
		t.Errorf("expected no control calls, got start=%d stop=%d enable=%v", rec.started, rec.stopped, rec.enableCalls)
	}

	data := serviceStateModel(t, resp.State)
	if data.Running.ValueBool() {
		t.Error("expected state running=false")
	}
}

func TestServiceResource_Create_EnableNotApplied(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{
		stoppedService(),
		{ID: 9, Name: "nfs", Enable: false, State: services.ServiceStateRunning},
	}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	assertServiceDiagnostic(t, resp.Diagnostics, "Service Enable Not Applied", "enable = false")

	// The start did land, so it has to reach state even though the apply failed.
	data := serviceStateModel(t, resp.State)
	if data.Enable.ValueBool() || !data.Running.ValueBool() {
		t.Errorf("expected state enable=false running=true, got %v/%v", data.Enable, data.Running)
	}
}

func TestServiceResource_Update_ServiceStaysRunning(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{runningService(), runningService()}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(
			serviceModelParams{ID: "nfs", Name: "nfs", Enable: true, Running: false})},
	}, resp)

	assertServiceDiagnostic(t, resp.Diagnostics, "Service Still Running", "running = false")
	if rec.stopped != 1 {
		t.Errorf("expected 1 stop, got %d", rec.stopped)
	}

	data := serviceStateModel(t, resp.State)
	if !data.Running.ValueBool() {
		t.Error("expected state to record the service as still running")
	}
}

// assertServiceDiagnostic checks the first error carries the given summary and
// names both the service and the state the appliance actually reported.
func assertServiceDiagnostic(t *testing.T, diags diag.Diagnostics, summary, detail string) {
	t.Helper()
	if !diags.HasError() {
		t.Fatal("expected error")
	}
	err := diags.Errors()[0]
	if err.Summary() != summary {
		t.Errorf("unexpected summary %q", err.Summary())
	}
	if !strings.Contains(err.Detail(), `"nfs"`) {
		t.Errorf("expected detail to name the service, got %q", err.Detail())
	}
	if !strings.Contains(err.Detail(), detail) {
		t.Errorf("expected detail to contain %q, got %q", detail, err.Detail())
	}
}

func serviceStateModel(t *testing.T, state tfsdk.State) ServiceResourceModel {
	t.Helper()
	if state.Raw.IsNull() {
		t.Fatal("expected state to be populated, got null")
	}
	var data ServiceResourceModel
	if diags := state.Get(context.Background(), &data); diags.HasError() {
		t.Fatalf("unable to read state: %v", diags)
	}
	return data
}

func TestServiceResource_Create_InvalidPlan(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: tftypes.Value{}},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid plan")
	}
}

func TestServiceResource_Read_Success(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(_ context.Context, name string) (*services.SystemService, error) {
			if name != "nfs" {
				t.Errorf("expected name 'nfs', got %q", name)
			}
			return runningService(), nil
		},
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(
			serviceModelParams{ID: "nfs", Name: "nfs", Enable: false, Running: false})},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var data ServiceResourceModel
	resp.State.Get(context.Background(), &data)
	if !data.Enable.ValueBool() || !data.Running.ValueBool() {
		t.Error("expected Read to refresh enable and running from the server")
	}
}

// A service the appliance no longer offers is removed from state rather than
// reported as an error, so the plan can drop it cleanly.
func TestServiceResource_Read_NotFound(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    createServiceModelValue(enabledServiceParams()),
	}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected resource to be removed from state")
	}
}

func TestServiceResource_Read_APIError(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			return nil, errors.New("boom")
		},
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestServiceResource_Read_InvalidState(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.Value{}},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid state")
	}
}

func TestServiceResource_Update_Success(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{runningService(), stoppedService()}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(
			serviceModelParams{ID: "nfs", Name: "nfs", Enable: false, Running: false})},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if len(rec.enableCalls) != 1 || rec.enableCalls[0] {
		t.Errorf("expected one SetEnable(false) call, got %v", rec.enableCalls)
	}
	if rec.stopped != 1 {
		t.Errorf("expected 1 stop, got %d", rec.stopped)
	}

	var data ServiceResourceModel
	resp.State.Get(context.Background(), &data)
	if data.Enable.ValueBool() || data.Running.ValueBool() {
		t.Error("expected enable and running false after update")
	}
}

func TestServiceResource_Update_APIError(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			return nil, errors.New("boom")
		},
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestServiceResource_Update_InvalidPlan(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: tftypes.Value{}},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid plan")
	}
}

// Destroy is stop-and-disable: the service itself belongs to the appliance.
func TestServiceResource_Delete_StopsAndDisables(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{runningService()}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if rec.stopped != 1 {
		t.Errorf("expected 1 stop, got %d", rec.stopped)
	}
	if len(rec.enableCalls) != 1 || rec.enableCalls[0] {
		t.Errorf("expected one SetEnable(false) call, got %v", rec.enableCalls)
	}
}

// A service already stopped and disabled needs no calls at all.
func TestServiceResource_Delete_AlreadyStopped(t *testing.T) {
	rec := &serviceRecorder{gets: []*services.SystemService{stoppedService()}}
	r := newServiceResource(rec.mock())

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if rec.stopped != 0 || len(rec.enableCalls) != 0 {
		t.Errorf("expected no control calls, got stop=%d enable=%v", rec.stopped, rec.enableCalls)
	}
}

// A service that has vanished server-side is already in the desired end state.
func TestServiceResource_Delete_NotFound(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestServiceResource_Delete_GetError(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			return nil, errors.New("boom")
		},
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestServiceResource_Delete_StopError(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			return runningService(), nil
		},
		StopFunc: func(context.Context, string) error { return errors.New("boom") },
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
	if summary := resp.Diagnostics.Errors()[0].Summary(); summary != "Unable to Stop Service" {
		t.Errorf("unexpected summary %q", summary)
	}
}

func TestServiceResource_Delete_DisableError(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{
		GetFunc: func(context.Context, string) (*services.SystemService, error) {
			return runningService(), nil
		},
		SetEnableFunc: func(context.Context, string, bool) error { return errors.New("boom") },
	})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createServiceModelValue(enabledServiceParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
	if summary := resp.Diagnostics.Errors()[0].Summary(); summary != "Unable to Disable Service" {
		t.Errorf("unexpected summary %q", summary)
	}
}

func TestServiceResource_Delete_InvalidState(t *testing.T) {
	r := newServiceResource(&services.MockSystemServices{})

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.Value{}},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid state")
	}
}

// Import takes the service name and has to seed `name` as well as `id`,
// otherwise the follow-up Read has nothing to query by.
func TestServiceResource_ImportState(t *testing.T) {
	r := NewServiceResource().(*ServiceResource)

	schemaResp := getServiceResourceSchema(t)
	resp := &resource.ImportStateResponse{State: tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    createServiceModelValue(serviceModelParams{}),
	}}

	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "nfs"}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var data ServiceResourceModel
	resp.State.Get(context.Background(), &data)
	if data.ID.ValueString() != "nfs" {
		t.Errorf("expected ID 'nfs', got %q", data.ID.ValueString())
	}
	if data.Name.ValueString() != "nfs" {
		t.Errorf("expected name 'nfs', got %q", data.Name.ValueString())
	}
}

func TestStartOrStop(t *testing.T) {
	if got := startOrStop(true); got != "start" {
		t.Errorf("expected 'start', got %q", got)
	}
	if got := startOrStop(false); got != "stop" {
		t.Errorf("expected 'stop', got %q", got)
	}
}
