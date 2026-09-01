package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestNewShareNFSResource(t *testing.T) {
	r := NewShareNFSResource()
	if r == nil {
		t.Fatal("NewShareNFSResource returned nil")
	}

	if _, ok := r.(*ShareNFSResource); !ok {
		t.Fatalf("expected *ShareNFSResource, got %T", r)
	}

	// Verify interface implementations
	_ = resource.Resource(r)
	_ = resource.ResourceWithConfigure(r.(*ShareNFSResource))
	_ = resource.ResourceWithImportState(r.(*ShareNFSResource))
}

func TestShareNFSResource_Metadata(t *testing.T) {
	r := NewShareNFSResource()

	req := resource.MetadataRequest{ProviderTypeName: "truenas"}
	resp := &resource.MetadataResponse{}

	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "truenas_share_nfs" {
		t.Errorf("expected TypeName 'truenas_share_nfs', got %q", resp.TypeName)
	}
}

func TestShareNFSResource_Configure_Success(t *testing.T) {
	r := NewShareNFSResource().(*ShareNFSResource)

	req := resource.ConfigureRequest{ProviderData: &services.TrueNASServices{}}
	resp := &resource.ConfigureResponse{}

	r.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestShareNFSResource_Configure_NilProviderData(t *testing.T) {
	r := NewShareNFSResource().(*ShareNFSResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestShareNFSResource_Configure_WrongType(t *testing.T) {
	r := NewShareNFSResource().(*ShareNFSResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "not a client"}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for wrong ProviderData type")
	}
}

func TestShareNFSResource_Schema(t *testing.T) {
	schemaResp := getShareNFSResourceSchema(t)

	if schemaResp.Schema.Description == "" {
		t.Error("expected non-empty schema description")
	}

	attrs := schemaResp.Schema.Attributes
	for _, name := range []string{
		"id", "path", "aliases", "comment", "networks", "hosts", "ro",
		"maproot_user", "maproot_group", "mapall_user", "mapall_group",
		"security", "enabled", "expose_snapshots", "locked",
	} {
		if attrs[name] == nil {
			t.Errorf("expected %q attribute", name)
		}
	}

	if !attrs["path"].IsRequired() {
		t.Error("expected 'path' to be required")
	}
	if !attrs["id"].IsComputed() {
		t.Error("expected 'id' to be computed")
	}
	if !attrs["locked"].IsComputed() || attrs["locked"].IsOptional() {
		t.Error("expected 'locked' to be computed-only")
	}
	if !attrs["maproot_user"].IsOptional() || attrs["maproot_user"].IsComputed() {
		t.Error("expected 'maproot_user' to be optional-only")
	}
	// TrueNAS discards any submitted aliases, so exposing it as settable would
	// guarantee an "inconsistent result after apply" error.
	if !attrs["aliases"].IsComputed() || attrs["aliases"].IsOptional() {
		t.Error("expected 'aliases' to be computed-only")
	}
}

func getShareNFSResourceSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	r := NewShareNFSResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}
	return *schemaResp
}

// shareNFSModelParams holds parameters for creating test model values.
// Nil list fields become null lists; interface{} scalars allow nil for null.
type shareNFSModelParams struct {
	ID              interface{}
	Path            interface{}
	Aliases         []string
	Networks        []string
	Hosts           []string
	Security        []string
	Comment         interface{}
	ReadOnly        interface{}
	MaprootUser     interface{}
	MaprootGroup    interface{}
	MapallUser      interface{}
	MapallGroup     interface{}
	Enabled         interface{}
	ExposeSnapshots interface{}
	Locked          interface{}
}

func stringListValue(items []string) tftypes.Value {
	listType := tftypes.List{ElementType: tftypes.String}
	if items == nil {
		return tftypes.NewValue(listType, nil)
	}

	elems := make([]tftypes.Value, len(items))
	for i, item := range items {
		elems[i] = tftypes.NewValue(tftypes.String, item)
	}
	return tftypes.NewValue(listType, elems)
}

// shareNFSValueWithUnknownHost builds an otherwise valid value whose hosts list
// contains an unknown element.
func shareNFSValueWithUnknownHost() tftypes.Value {
	listType := tftypes.List{ElementType: tftypes.String}
	params := fullShareNFSParams()
	params.ID = "7"

	value := createShareNFSModelValue(params)

	var attrs map[string]tftypes.Value
	if err := value.As(&attrs); err != nil {
		panic(err)
	}
	attrs["hosts"] = tftypes.NewValue(listType, []tftypes.Value{
		tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})

	return tftypes.NewValue(value.Type(), attrs)
}

func createShareNFSModelValue(p shareNFSModelParams) tftypes.Value {
	listType := tftypes.List{ElementType: tftypes.String}

	objectType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":               tftypes.String,
			"path":             tftypes.String,
			"aliases":          listType,
			"comment":          tftypes.String,
			"networks":         listType,
			"hosts":            listType,
			"ro":               tftypes.Bool,
			"maproot_user":     tftypes.String,
			"maproot_group":    tftypes.String,
			"mapall_user":      tftypes.String,
			"mapall_group":     tftypes.String,
			"security":         listType,
			"enabled":          tftypes.Bool,
			"expose_snapshots": tftypes.Bool,
			"locked":           tftypes.Bool,
		},
	}

	return tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":               tftypes.NewValue(tftypes.String, p.ID),
		"path":             tftypes.NewValue(tftypes.String, p.Path),
		"aliases":          stringListValue(p.Aliases),
		"comment":          tftypes.NewValue(tftypes.String, p.Comment),
		"networks":         stringListValue(p.Networks),
		"hosts":            stringListValue(p.Hosts),
		"ro":               tftypes.NewValue(tftypes.Bool, p.ReadOnly),
		"maproot_user":     tftypes.NewValue(tftypes.String, p.MaprootUser),
		"maproot_group":    tftypes.NewValue(tftypes.String, p.MaprootGroup),
		"mapall_user":      tftypes.NewValue(tftypes.String, p.MapallUser),
		"mapall_group":     tftypes.NewValue(tftypes.String, p.MapallGroup),
		"security":         stringListValue(p.Security),
		"enabled":          tftypes.NewValue(tftypes.Bool, p.Enabled),
		"expose_snapshots": tftypes.NewValue(tftypes.Bool, p.ExposeSnapshots),
		"locked":           tftypes.NewValue(tftypes.Bool, p.Locked),
	})
}

// testNFSShare returns a standard test share for use in tests.
func testNFSShare() *services.NFSShare {
	return &services.NFSShare{
		ID:              7,
		Path:            "/mnt/tank/media",
		Aliases:         []string{},
		Comment:         "Media library",
		Networks:        []string{"10.0.0.0/24"},
		Hosts:           []string{"nas-client"},
		ReadOnly:        true,
		MaprootUser:     strPtr("root"),
		MaprootGroup:    strPtr("root"),
		Security:        []string{"SYS"},
		Enabled:         true,
		ExposeSnapshots: false,
		Locked:          boolPtr(false),
	}
}

// fullShareNFSParams matches testNFSShare, for use as plan/state input.
func fullShareNFSParams() shareNFSModelParams {
	return shareNFSModelParams{
		Path:            "/mnt/tank/media",
		Aliases:         []string{},
		Comment:         "Media library",
		Networks:        []string{"10.0.0.0/24"},
		Hosts:           []string{"nas-client"},
		ReadOnly:        true,
		MaprootUser:     "root",
		MaprootGroup:    "root",
		Security:        []string{"SYS"},
		Enabled:         true,
		ExposeSnapshots: false,
		Locked:          false,
	}
}

func assertShareNFSState(t *testing.T, data ShareNFSResourceModel) {
	t.Helper()

	if data.ID.ValueString() != "7" {
		t.Errorf("expected ID '7', got %q", data.ID.ValueString())
	}
	if data.Path.ValueString() != "/mnt/tank/media" {
		t.Errorf("expected path '/mnt/tank/media', got %q", data.Path.ValueString())
	}
	if data.Comment.ValueString() != "Media library" {
		t.Errorf("expected comment 'Media library', got %q", data.Comment.ValueString())
	}
	if !data.ReadOnly.ValueBool() {
		t.Error("expected ro true")
	}
	if !data.Enabled.ValueBool() {
		t.Error("expected enabled true")
	}
	if data.MaprootUser.ValueString() != "root" {
		t.Errorf("expected maproot_user 'root', got %q", data.MaprootUser.ValueString())
	}
	if !data.MapallUser.IsNull() {
		t.Error("expected mapall_user to be null")
	}
	if data.Locked.ValueBool() {
		t.Error("expected locked false")
	}
	if len(data.Networks.Elements()) != 1 {
		t.Errorf("expected 1 network, got %d", len(data.Networks.Elements()))
	}
	if len(data.Aliases.Elements()) != 0 {
		t.Errorf("expected 0 aliases, got %d", len(data.Aliases.Elements()))
	}
}

func newShareNFSResource(mock *services.MockSharingNFSService) *ShareNFSResource {
	return &ShareNFSResource{
		BaseResource: BaseResource{services: &services.TrueNASServices{SharingNFS: mock}},
	}
}

func TestShareNFSResource_Create_Success(t *testing.T) {
	var capturedOpts services.CreateNFSShareOpts

	r := newShareNFSResource(&services.MockSharingNFSService{
		CreateFunc: func(ctx context.Context, opts services.CreateNFSShareOpts) (*services.NFSShare, error) {
			capturedOpts = opts
			return testNFSShare(), nil
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createShareNFSModelValue(fullShareNFSParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if capturedOpts.Path != "/mnt/tank/media" {
		t.Errorf("expected path '/mnt/tank/media', got %q", capturedOpts.Path)
	}
	if capturedOpts.Comment != "Media library" {
		t.Errorf("expected comment 'Media library', got %q", capturedOpts.Comment)
	}
	if !capturedOpts.ReadOnly {
		t.Error("expected ReadOnly true")
	}
	if !capturedOpts.Enabled {
		t.Error("expected Enabled true")
	}
	if capturedOpts.MaprootUser == nil || *capturedOpts.MaprootUser != "root" {
		t.Errorf("expected MaprootUser 'root', got %v", capturedOpts.MaprootUser)
	}
	if capturedOpts.MapallUser != nil {
		t.Errorf("expected MapallUser nil, got %v", *capturedOpts.MapallUser)
	}
	if len(capturedOpts.Networks) != 1 || capturedOpts.Networks[0] != "10.0.0.0/24" {
		t.Errorf("unexpected networks: %v", capturedOpts.Networks)
	}
	if len(capturedOpts.Security) != 1 || capturedOpts.Security[0] != "SYS" {
		t.Errorf("unexpected security: %v", capturedOpts.Security)
	}
	var data ShareNFSResourceModel
	resp.State.Get(context.Background(), &data)
	assertShareNFSState(t, data)
}

func TestShareNFSResource_Create_NullListsBecomeEmpty(t *testing.T) {
	var capturedOpts services.CreateNFSShareOpts

	r := newShareNFSResource(&services.MockSharingNFSService{
		CreateFunc: func(ctx context.Context, opts services.CreateNFSShareOpts) (*services.NFSShare, error) {
			capturedOpts = opts
			return testNFSShare(), nil
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createShareNFSModelValue(shareNFSModelParams{
			Path:    "/mnt/tank/media",
			Comment: "",
		})},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	for name, got := range map[string][]string{
		"networks": capturedOpts.Networks,
		"hosts":    capturedOpts.Hosts,
		"security": capturedOpts.Security,
	} {
		if got == nil {
			t.Errorf("expected empty (non-nil) slice for %s", name)
		}
		if len(got) != 0 {
			t.Errorf("expected empty slice for %s, got %v", name, got)
		}
	}

	if capturedOpts.MaprootUser != nil {
		t.Error("expected nil MaprootUser for null attribute")
	}
}

func TestShareNFSResource_Create_APIError(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{
		CreateFunc: func(ctx context.Context, opts services.CreateNFSShareOpts) (*services.NFSShare, error) {
			return nil, errors.New("connection refused")
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createShareNFSModelValue(fullShareNFSParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to not be set when API returns error")
	}
}

func TestShareNFSResource_Create_NilShare(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{
		CreateFunc: func(ctx context.Context, opts services.CreateNFSShareOpts) (*services.NFSShare, error) {
			return nil, nil
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createShareNFSModelValue(fullShareNFSParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error when share is nil")
	}
}

// An unknown list element cannot be converted to a string, so the guard after
// buildShareNFSOpts must surface the diagnostic instead of calling the API.
func TestShareNFSResource_Create_UnknownListElement(t *testing.T) {
	called := false
	r := newShareNFSResource(&services.MockSharingNFSService{
		CreateFunc: func(ctx context.Context, opts services.CreateNFSShareOpts) (*services.NFSShare, error) {
			called = true
			return testNFSShare(), nil
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: shareNFSValueWithUnknownHost()},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unknown list element")
	}
	if called {
		t.Error("expected Create not to reach the API when conversion fails")
	}
}

func TestShareNFSResource_Update_UnknownListElement(t *testing.T) {
	called := false
	r := newShareNFSResource(&services.MockSharingNFSService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateNFSShareOpts) (*services.NFSShare, error) {
			called = true
			return testNFSShare(), nil
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	stateParams := fullShareNFSParams()
	stateParams.ID = "7"

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createShareNFSModelValue(stateParams)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: shareNFSValueWithUnknownHost()},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unknown list element")
	}
	if called {
		t.Error("expected Update not to reach the API when conversion fails")
	}
}

func TestShareNFSResource_Create_InvalidPlan(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{})

	schemaResp := getShareNFSResourceSchema(t)
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

func TestShareNFSResource_Read_Success(t *testing.T) {
	var capturedID int64

	r := newShareNFSResource(&services.MockSharingNFSService{
		GetFunc: func(ctx context.Context, id int64) (*services.NFSShare, error) {
			capturedID = id
			return testNFSShare(), nil
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "7"
	stateValue := createShareNFSModelValue(params)

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 7 {
		t.Errorf("expected id 7, got %d", capturedID)
	}

	var data ShareNFSResourceModel
	resp.State.Get(context.Background(), &data)
	assertShareNFSState(t, data)
}

func TestShareNFSResource_Read_NotFound(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{
		GetFunc: func(ctx context.Context, id int64) (*services.NFSShare, error) { return nil, nil },
	})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "7"
	stateValue := createShareNFSModelValue(params)

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed when share no longer exists")
	}
}

func TestShareNFSResource_Read_APIError(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{
		GetFunc: func(ctx context.Context, id int64) (*services.NFSShare, error) {
			return nil, errors.New("connection refused")
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "7"
	stateValue := createShareNFSModelValue(params)

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestShareNFSResource_Read_InvalidID(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "not-a-number"
	stateValue := createShareNFSModelValue(params)

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestShareNFSResource_Read_InvalidState(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{})

	schemaResp := getShareNFSResourceSchema(t)
	badValue := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: badValue}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: badValue},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable state")
	}
}

func TestShareNFSResource_Update_Success(t *testing.T) {
	var capturedID int64
	var capturedOpts services.UpdateNFSShareOpts

	updated := testNFSShare()
	updated.Comment = "Updated comment"
	updated.ReadOnly = false

	r := newShareNFSResource(&services.MockSharingNFSService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateNFSShareOpts) (*services.NFSShare, error) {
			capturedID = id
			capturedOpts = opts
			return updated, nil
		},
	})

	schemaResp := getShareNFSResourceSchema(t)

	stateParams := fullShareNFSParams()
	stateParams.ID = "7"

	planParams := fullShareNFSParams()
	planParams.ID = "7"
	planParams.Comment = "Updated comment"
	planParams.ReadOnly = false

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createShareNFSModelValue(stateParams)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: createShareNFSModelValue(planParams)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 7 {
		t.Errorf("expected id 7, got %d", capturedID)
	}
	if capturedOpts.Comment != "Updated comment" {
		t.Errorf("expected updated comment, got %q", capturedOpts.Comment)
	}
	if capturedOpts.ReadOnly {
		t.Error("expected ReadOnly false in update opts")
	}

	var data ShareNFSResourceModel
	resp.State.Get(context.Background(), &data)
	if data.Comment.ValueString() != "Updated comment" {
		t.Errorf("expected updated comment in state, got %q", data.Comment.ValueString())
	}
	if data.ReadOnly.ValueBool() {
		t.Error("expected ro false in state")
	}
}

func TestShareNFSResource_Update_APIError(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateNFSShareOpts) (*services.NFSShare, error) {
			return nil, errors.New("connection refused")
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "7"
	value := createShareNFSModelValue(params)

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestShareNFSResource_Update_NilShare(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateNFSShareOpts) (*services.NFSShare, error) {
			return nil, nil
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "7"
	value := createShareNFSModelValue(params)

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error when share is nil")
	}
}

func TestShareNFSResource_Update_InvalidID(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "not-a-number"
	value := createShareNFSModelValue(params)

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestShareNFSResource_Update_InvalidState(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{})

	schemaResp := getShareNFSResourceSchema(t)
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

func TestShareNFSResource_Delete_Success(t *testing.T) {
	var capturedID int64
	called := false

	r := newShareNFSResource(&services.MockSharingNFSService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			called = true
			capturedID = id
			return nil
		},
	})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "7"
	value := createShareNFSModelValue(params)

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
	if capturedID != 7 {
		t.Errorf("expected id 7, got %d", capturedID)
	}
}

func TestShareNFSResource_Delete_APIError(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{
		DeleteFunc: func(ctx context.Context, id int64) error { return errors.New("connection refused") },
	})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "7"
	value := createShareNFSModelValue(params)

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: value}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestShareNFSResource_Delete_InvalidID(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{})

	schemaResp := getShareNFSResourceSchema(t)
	params := fullShareNFSParams()
	params.ID = "not-a-number"
	value := createShareNFSModelValue(params)

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: value}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestShareNFSResource_Delete_InvalidState(t *testing.T) {
	r := newShareNFSResource(&services.MockSharingNFSService{})

	schemaResp := getShareNFSResourceSchema(t)
	badValue := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: badValue}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: badValue},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable state")
	}
}

func TestMapShareNFSToModel_NullLocked(t *testing.T) {
	share := testNFSShare()
	share.Locked = nil
	share.MaprootUser = nil
	share.MaprootGroup = nil

	var data ShareNFSResourceModel
	mapShareNFSToModel(share, &data)

	if !data.Locked.IsNull() {
		t.Error("expected locked to be null when API returns null")
	}
	if !data.MaprootUser.IsNull() {
		t.Error("expected maproot_user to be null")
	}
	if !data.MaprootGroup.IsNull() {
		t.Error("expected maproot_group to be null")
	}
}

func TestNFSOptionalString(t *testing.T) {
	if got := nfsOptionalString(types.StringNull()); got != nil {
		t.Errorf("expected nil for null, got %q", *got)
	}
	if got := nfsOptionalString(types.StringUnknown()); got != nil {
		t.Errorf("expected nil for unknown, got %q", *got)
	}
	if got := nfsOptionalString(types.StringValue("nobody")); got == nil || *got != "nobody" {
		t.Errorf("expected 'nobody', got %v", got)
	}
}

func TestNFSStringPointerValue(t *testing.T) {
	if got := nfsStringPointerValue(nil); !got.IsNull() {
		t.Error("expected null for nil pointer")
	}
	if got := nfsStringPointerValue(strPtr("root")); got.ValueString() != "root" {
		t.Errorf("expected 'root', got %q", got.ValueString())
	}
}

func TestNFSStringsFromList(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	if got := nfsStringsFromList(ctx, types.ListNull(types.StringType), &diags); len(got) != 0 || got == nil {
		t.Errorf("expected empty non-nil slice for null list, got %v", got)
	}
	if got := nfsStringsFromList(ctx, types.ListUnknown(types.StringType), &diags); len(got) != 0 || got == nil {
		t.Errorf("expected empty non-nil slice for unknown list, got %v", got)
	}

	list, _ := types.ListValueFrom(ctx, types.StringType, []string{"a", "b"})
	got := nfsStringsFromList(ctx, list, &diags)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("unexpected slice: %v", got)
	}
	if diags.HasError() {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
}

func TestNFSListFromStrings(t *testing.T) {
	list := nfsListFromStrings([]string{"SYS", "KRB5"})
	if len(list.Elements()) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(list.Elements()))
	}
	if got := list.Elements()[0].(types.String).ValueString(); got != "SYS" {
		t.Errorf("expected first element 'SYS', got %q", got)
	}

	empty := nfsListFromStrings([]string{})
	if empty.IsNull() || len(empty.Elements()) != 0 {
		t.Errorf("expected empty list, got %v", empty)
	}
}

func TestEmptyStringList(t *testing.T) {
	list := emptyStringList()
	if list.IsNull() || list.IsUnknown() {
		t.Error("expected known, non-null list")
	}
	if len(list.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(list.Elements()))
	}
}
