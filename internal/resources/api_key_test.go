package resources

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const testAPIKeySecret = "2-amfpg5iQSKI5rClsylOGH09wZnCvcmsKUJdGxu9yUddRkQWewAW21fgLwdowOiVy"

func TestNewAPIKeyResource(t *testing.T) {
	r := NewAPIKeyResource()
	if r == nil {
		t.Fatal("NewAPIKeyResource returned nil")
	}

	if _, ok := r.(*APIKeyResource); !ok {
		t.Fatalf("expected *APIKeyResource, got %T", r)
	}

	// Verify interface implementations
	_ = resource.Resource(r)
	_ = resource.ResourceWithConfigure(r.(*APIKeyResource))
	_ = resource.ResourceWithImportState(r.(*APIKeyResource))
}

func TestAPIKeyResource_Metadata(t *testing.T) {
	r := NewAPIKeyResource()

	req := resource.MetadataRequest{ProviderTypeName: "truenas"}
	resp := &resource.MetadataResponse{}

	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "truenas_api_key" {
		t.Errorf("expected TypeName 'truenas_api_key', got %q", resp.TypeName)
	}
}

func TestAPIKeyResource_Configure_Success(t *testing.T) {
	r := NewAPIKeyResource().(*APIKeyResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &services.TrueNASServices{}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestAPIKeyResource_Configure_NilProviderData(t *testing.T) {
	r := NewAPIKeyResource().(*APIKeyResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
}

func TestAPIKeyResource_Configure_WrongType(t *testing.T) {
	r := NewAPIKeyResource().(*APIKeyResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "not a client"}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for wrong ProviderData type")
	}
}

func TestAPIKeyResource_Schema(t *testing.T) {
	schemaResp := getAPIKeyResourceSchema(t)

	if schemaResp.Schema.Description == "" {
		t.Error("expected non-empty schema description")
	}

	attrs := schemaResp.Schema.Attributes
	for _, name := range []string{
		"id", "name", "username", "expires_at", "store_key", "key",
		"user_identifier", "created_at", "local", "revoked", "revoked_reason",
	} {
		if attrs[name] == nil {
			t.Errorf("expected %q attribute", name)
		}
	}

	if !attrs["name"].IsRequired() {
		t.Error("expected 'name' to be required")
	}
	if !attrs["username"].IsRequired() {
		t.Error("expected 'username' to be required")
	}
	if !attrs["expires_at"].IsOptional() || attrs["expires_at"].IsComputed() {
		t.Error("expected 'expires_at' to be optional-only")
	}
	if !attrs["store_key"].IsOptional() || !attrs["store_key"].IsComputed() {
		t.Error("expected 'store_key' to be optional and computed")
	}
	// The secret must never be rendered in plan output or logs.
	if !attrs["key"].IsSensitive() {
		t.Error("expected 'key' to be sensitive")
	}
	if !attrs["key"].IsComputed() || attrs["key"].IsOptional() {
		t.Error("expected 'key' to be computed-only")
	}
	for _, name := range []string{"user_identifier", "created_at", "local", "revoked", "revoked_reason"} {
		if !attrs[name].IsComputed() || attrs[name].IsOptional() {
			t.Errorf("expected %q to be computed-only", name)
		}
	}
}

func getAPIKeyResourceSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	r := NewAPIKeyResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}
	return *schemaResp
}

// apiKeyModelParams holds parameters for creating test model values.
// interface{} scalars allow nil for null.
type apiKeyModelParams struct {
	ID             interface{}
	Name           interface{}
	Username       interface{}
	ExpiresAt      interface{}
	StoreKey       interface{}
	Key            interface{}
	UserIdentifier interface{}
	CreatedAt      interface{}
	Local          interface{}
	Revoked        interface{}
	RevokedReason  interface{}
}

func createAPIKeyModelValue(p apiKeyModelParams) tftypes.Value {
	objectType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":              tftypes.String,
			"name":            tftypes.String,
			"username":        tftypes.String,
			"expires_at":      tftypes.String,
			"store_key":       tftypes.Bool,
			"key":             tftypes.String,
			"user_identifier": tftypes.String,
			"created_at":      tftypes.String,
			"local":           tftypes.Bool,
			"revoked":         tftypes.Bool,
			"revoked_reason":  tftypes.String,
		},
	}

	return tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":              tftypes.NewValue(tftypes.String, p.ID),
		"name":            tftypes.NewValue(tftypes.String, p.Name),
		"username":        tftypes.NewValue(tftypes.String, p.Username),
		"expires_at":      tftypes.NewValue(tftypes.String, p.ExpiresAt),
		"store_key":       tftypes.NewValue(tftypes.Bool, p.StoreKey),
		"key":             tftypes.NewValue(tftypes.String, p.Key),
		"user_identifier": tftypes.NewValue(tftypes.String, p.UserIdentifier),
		"created_at":      tftypes.NewValue(tftypes.String, p.CreatedAt),
		"local":           tftypes.NewValue(tftypes.Bool, p.Local),
		"revoked":         tftypes.NewValue(tftypes.Bool, p.Revoked),
		"revoked_reason":  tftypes.NewValue(tftypes.String, p.RevokedReason),
	})
}

// testAPIKey returns the key the API reports for the standard test config.
// Key is set as only a create reply would set it.
func testAPIKey() *services.APIKey {
	username := "terraform"
	expiresAt := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)

	return &services.APIKey{
		ID:             2,
		Name:           "terraform",
		Username:       &username,
		UserIdentifier: "33",
		CreatedAt:      time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC),
		ExpiresAt:      &expiresAt,
		Local:          true,
		Revoked:        false,
		Key:            testAPIKeySecret,
	}
}

// fullAPIKeyParams matches testAPIKey, for use as plan or state input.
func fullAPIKeyParams() apiKeyModelParams {
	return apiKeyModelParams{
		Name:      "terraform",
		Username:  "terraform",
		ExpiresAt: "2030-01-02T03:04:05Z",
		StoreKey:  true,
	}
}

func assertAPIKeyState(t *testing.T, data APIKeyResourceModel) {
	t.Helper()

	if data.ID.ValueString() != "2" {
		t.Errorf("expected ID '2', got %q", data.ID.ValueString())
	}
	if data.Name.ValueString() != "terraform" {
		t.Errorf("expected name 'terraform', got %q", data.Name.ValueString())
	}
	if data.Username.ValueString() != "terraform" {
		t.Errorf("expected username 'terraform', got %q", data.Username.ValueString())
	}
	if data.ExpiresAt.ValueString() != "2030-01-02T03:04:05Z" {
		t.Errorf("expected expires_at '2030-01-02T03:04:05Z', got %q", data.ExpiresAt.ValueString())
	}
	if data.UserIdentifier.ValueString() != "33" {
		t.Errorf("expected user_identifier '33', got %q", data.UserIdentifier.ValueString())
	}
	if data.CreatedAt.ValueString() != "2026-09-01T12:00:00Z" {
		t.Errorf("expected created_at '2026-09-01T12:00:00Z', got %q", data.CreatedAt.ValueString())
	}
	if !data.Local.ValueBool() {
		t.Error("expected local true")
	}
	if data.Revoked.ValueBool() {
		t.Error("expected revoked false")
	}
	if !data.RevokedReason.IsNull() {
		t.Error("expected revoked_reason to be null")
	}
}

func newAPIKeyResource(mock *services.MockAPIKeyService) *APIKeyResource {
	return &APIKeyResource{
		BaseResource: BaseResource{services: &services.TrueNASServices{APIKey: mock}},
	}
}

func TestAPIKeyResource_Create_Success(t *testing.T) {
	var capturedOpts services.CreateAPIKeyOpts

	r := newAPIKeyResource(&services.MockAPIKeyService{
		CreateFunc: func(ctx context.Context, opts services.CreateAPIKeyOpts) (*services.APIKey, error) {
			capturedOpts = opts
			return testAPIKey(), nil
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(fullAPIKeyParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if capturedOpts.Name != "terraform" {
		t.Errorf("expected name 'terraform', got %q", capturedOpts.Name)
	}
	if capturedOpts.Username != "terraform" {
		t.Errorf("expected username 'terraform', got %q", capturedOpts.Username)
	}
	want := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	if capturedOpts.ExpiresAt == nil || !capturedOpts.ExpiresAt.Equal(want) {
		t.Errorf("expected expires_at %s, got %v", want, capturedOpts.ExpiresAt)
	}

	var data APIKeyResourceModel
	resp.State.Get(context.Background(), &data)
	assertAPIKeyState(t, data)

	if data.Key.ValueString() != testAPIKeySecret {
		t.Errorf("expected the secret in state, got %q", data.Key.ValueString())
	}
	if !data.StoreKey.ValueBool() {
		t.Error("expected store_key true")
	}
}

func TestAPIKeyResource_Create_NoExpiry(t *testing.T) {
	var capturedOpts services.CreateAPIKeyOpts

	created := testAPIKey()
	created.ExpiresAt = nil

	r := newAPIKeyResource(&services.MockAPIKeyService{
		CreateFunc: func(ctx context.Context, opts services.CreateAPIKeyOpts) (*services.APIKey, error) {
			capturedOpts = opts
			return created, nil
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	params := fullAPIKeyParams()
	params.ExpiresAt = nil

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedOpts.ExpiresAt != nil {
		t.Errorf("expected nil expires_at, got %s", capturedOpts.ExpiresAt)
	}

	var data APIKeyResourceModel
	resp.State.Get(context.Background(), &data)
	if !data.ExpiresAt.IsNull() {
		t.Errorf("expected expires_at to be null, got %q", data.ExpiresAt.ValueString())
	}
}

// The secret reaches state even when store_key is false, so resources later in
// the creating apply can consume it. Read drops it on the next refresh.
func TestAPIKeyResource_Create_StoreKeyFalseStillReturnsSecret(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{
		CreateFunc: func(ctx context.Context, opts services.CreateAPIKeyOpts) (*services.APIKey, error) {
			return testAPIKey(), nil
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	params := fullAPIKeyParams()
	params.StoreKey = false

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var data APIKeyResourceModel
	resp.State.Get(context.Background(), &data)
	if data.Key.ValueString() != testAPIKeySecret {
		t.Errorf("expected the secret to be available during the creating apply, got %q", data.Key.ValueString())
	}
}

func TestAPIKeyResource_Create_APIError(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{
		CreateFunc: func(ctx context.Context, opts services.CreateAPIKeyOpts) (*services.APIKey, error) {
			return nil, errors.New("boom")
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(fullAPIKeyParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestAPIKeyResource_Create_NilKey(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(fullAPIKeyParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for a nil key")
	}
}

func TestAPIKeyResource_Create_InvalidExpiry(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{
		CreateFunc: func(ctx context.Context, opts services.CreateAPIKeyOpts) (*services.APIKey, error) {
			t.Fatal("create must not be called with an unparseable expiry")
			return nil, nil
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	params := fullAPIKeyParams()
	params.ExpiresAt = "tomorrow"

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for an unparseable expiry")
	}
}

func TestAPIKeyResource_Create_InvalidPlan(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(context.Background(), resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: tftypes.Value{}},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for an invalid plan")
	}
}

func TestAPIKeyResource_Read_Success(t *testing.T) {
	var capturedID int64

	read := testAPIKey()
	read.Key = "" // the API never discloses the secret again

	r := newAPIKeyResource(&services.MockAPIKeyService{
		GetFunc: func(ctx context.Context, id int64) (*services.APIKey, error) {
			capturedID = id
			return read, nil
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	params := fullAPIKeyParams()
	params.ID = "2"
	params.Key = testAPIKeySecret

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 2 {
		t.Errorf("expected ID 2, got %d", capturedID)
	}

	var data APIKeyResourceModel
	resp.State.Get(context.Background(), &data)
	assertAPIKeyState(t, data)

	if data.Key.ValueString() != testAPIKeySecret {
		t.Errorf("expected the stored secret to survive a refresh, got %q", data.Key.ValueString())
	}
}

// With store_key false, the refresh after the creating apply is where the
// secret leaves state.
func TestAPIKeyResource_Read_StoreKeyFalseDropsSecret(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{
		GetFunc: func(ctx context.Context, id int64) (*services.APIKey, error) {
			return testAPIKey(), nil
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	params := fullAPIKeyParams()
	params.ID = "2"
	params.Key = testAPIKeySecret
	params.StoreKey = false

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var data APIKeyResourceModel
	resp.State.Get(context.Background(), &data)
	if !data.Key.IsNull() {
		t.Errorf("expected the secret to be dropped from state, got %q", data.Key.ValueString())
	}
}

// An import has no store_key in state; it takes the schema default so the
// first plan after the import is empty.
func TestAPIKeyResource_Read_ImportedKeyDefaultsStoreKey(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{
		GetFunc: func(ctx context.Context, id int64) (*services.APIKey, error) {
			imported := testAPIKey()
			imported.Key = ""
			return imported, nil
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(apiKeyModelParams{ID: "2"})},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var data APIKeyResourceModel
	resp.State.Get(context.Background(), &data)
	if !data.StoreKey.ValueBool() {
		t.Error("expected store_key to default to true after an import")
	}
	if !data.Key.IsNull() {
		t.Errorf("expected an imported key to have no secret, got %q", data.Key.ValueString())
	}
}

func TestAPIKeyResource_Read_NotFound(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	schemaResp := getAPIKeyResourceSchema(t)
	params := fullAPIKeyParams()
	params.ID = "2"

	resp := &resource.ReadResponse{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected the resource to be removed from state")
	}
}

func TestAPIKeyResource_Read_APIError(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{
		GetFunc: func(ctx context.Context, id int64) (*services.APIKey, error) {
			return nil, errors.New("boom")
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	params := fullAPIKeyParams()
	params.ID = "2"

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestAPIKeyResource_Read_InvalidID(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	params := fullAPIKeyParams()
	params.ID = "not-a-number"

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for an unparseable ID")
	}
}

func TestAPIKeyResource_Read_InvalidState(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.Value{}},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid state")
	}
}

func TestAPIKeyResource_Update_Success(t *testing.T) {
	var capturedID int64
	var capturedOpts services.UpdateAPIKeyOpts

	updated := testAPIKey()
	updated.Name = "renamed"
	updated.Key = "" // the API never discloses the secret on update

	r := newAPIKeyResource(&services.MockAPIKeyService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateAPIKeyOpts) (*services.APIKey, error) {
			capturedID = id
			capturedOpts = opts
			return updated, nil
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	state := fullAPIKeyParams()
	state.ID = "2"
	state.Key = testAPIKeySecret

	plan := state
	plan.Name = "renamed"

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(state)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(plan)},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 2 {
		t.Errorf("expected ID 2, got %d", capturedID)
	}
	if capturedOpts.Name != "renamed" {
		t.Errorf("expected name 'renamed', got %q", capturedOpts.Name)
	}

	var data APIKeyResourceModel
	resp.State.Get(context.Background(), &data)
	if data.Name.ValueString() != "renamed" {
		t.Errorf("expected name 'renamed', got %q", data.Name.ValueString())
	}
	if data.Key.ValueString() != testAPIKeySecret {
		t.Errorf("expected the stored secret to survive an update, got %q", data.Key.ValueString())
	}
}

func TestAPIKeyResource_Update_APIError(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateAPIKeyOpts) (*services.APIKey, error) {
			return nil, errors.New("boom")
		},
	})

	if diags := updateAPIKeyWithParams(t, r, updatableAPIKeyParams()); !diags {
		t.Fatal("expected error")
	}
}

func TestAPIKeyResource_Update_NilKey(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	if diags := updateAPIKeyWithParams(t, r, updatableAPIKeyParams()); !diags {
		t.Fatal("expected error for a nil key")
	}
}

func TestAPIKeyResource_Update_InvalidID(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	params := updatableAPIKeyParams()
	params.ID = "not-a-number"

	if diags := updateAPIKeyWithParams(t, r, params); !diags {
		t.Fatal("expected error for an unparseable ID")
	}
}

func TestAPIKeyResource_Update_InvalidExpiry(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateAPIKeyOpts) (*services.APIKey, error) {
			t.Fatal("update must not be called with an unparseable expiry")
			return nil, nil
		},
	})

	params := updatableAPIKeyParams()
	params.ExpiresAt = "tomorrow"

	if diags := updateAPIKeyWithParams(t, r, params); !diags {
		t.Fatal("expected error for an unparseable expiry")
	}
}

func TestAPIKeyResource_Update_InvalidState(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.Value{}},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: tftypes.Value{}},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid state")
	}
}

// updatableAPIKeyParams returns state that Update can be driven with.
func updatableAPIKeyParams() apiKeyModelParams {
	params := fullAPIKeyParams()
	params.ID = "2"
	return params
}

// updateAPIKeyWithParams drives Update with identical state and plan, and
// reports whether it produced errors.
func updateAPIKeyWithParams(t *testing.T, r *APIKeyResource, params apiKeyModelParams) bool {
	t.Helper()

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	return resp.Diagnostics.HasError()
}

func TestAPIKeyResource_Delete_Success(t *testing.T) {
	var capturedID int64
	called := false

	r := newAPIKeyResource(&services.MockAPIKeyService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			capturedID = id
			called = true
			return nil
		},
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(updatableAPIKeyParams())},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !called {
		t.Fatal("expected Delete to be called")
	}
	if capturedID != 2 {
		t.Errorf("expected ID 2, got %d", capturedID)
	}
}

func TestAPIKeyResource_Delete_APIError(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{
		DeleteFunc: func(ctx context.Context, id int64) error { return errors.New("boom") },
	})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(updatableAPIKeyParams())},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
}

func TestAPIKeyResource_Delete_InvalidID(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	params := updatableAPIKeyParams()
	params.ID = "not-a-number"

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: createAPIKeyModelValue(params)},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for an unparseable ID")
	}
}

func TestAPIKeyResource_Delete_InvalidState(t *testing.T) {
	r := newAPIKeyResource(&services.MockAPIKeyService{})

	schemaResp := getAPIKeyResourceSchema(t)
	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.Value{}},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid state")
	}
}

// The plan carries the stored secret forward across an update; nothing but a
// creation can put a new one in state.
func TestAPIKeyResource_Schema_KeyUsesStateForUnknown(t *testing.T) {
	attr, ok := getAPIKeyResourceSchema(t).Schema.Attributes["key"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected a StringAttribute for 'key'")
	}
	if len(attr.PlanModifiers) == 0 {
		t.Fatal("expected 'key' to keep its state value across an update")
	}

	resp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	attr.PlanModifiers[0].PlanModifyString(context.Background(), planmodifier.StringRequest{
		State:       tfsdk.State{Schema: getAPIKeyResourceSchema(t).Schema, Raw: createAPIKeyModelValue(updatableAPIKeyParams())},
		StateValue:  types.StringValue(testAPIKeySecret),
		PlanValue:   types.StringUnknown(),
		ConfigValue: types.StringNull(),
	}, resp)

	if resp.PlanValue.ValueString() != testAPIKeySecret {
		t.Errorf("expected the stored secret to be planned, got %v", resp.PlanValue)
	}
}

func TestRequiresReplaceWhenStoringKeyAgain(t *testing.T) {
	tests := map[string]struct {
		state  bool
		config types.Bool
		plan   bool
		want   bool
	}{
		"storage turned back on":        {state: false, config: types.BoolValue(true), plan: true, want: true},
		"storage turned off":            {state: true, config: types.BoolValue(false), plan: false, want: false},
		"storage stays on":              {state: true, config: types.BoolValue(true), plan: true, want: false},
		"storage stays off":             {state: false, config: types.BoolValue(false), plan: false, want: false},
		"store_key dropped from config": {state: false, config: types.BoolNull(), plan: true, want: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resp := &boolplanmodifier.RequiresReplaceIfFuncResponse{}
			requiresReplaceWhenStoringKeyAgain(context.Background(), planmodifier.BoolRequest{
				StateValue:  types.BoolValue(tt.state),
				ConfigValue: tt.config,
				PlanValue:   types.BoolValue(tt.plan),
			}, resp)

			if resp.RequiresReplace != tt.want {
				t.Errorf("expected %v, got %v", tt.want, resp.RequiresReplace)
			}
		})
	}
}

// A system key has no owning username; the model reports it as null rather
// than inventing one.
func TestMapAPIKeyToModel_NullUsername(t *testing.T) {
	apiKey := testAPIKey()
	apiKey.Username = nil
	apiKey.ExpiresAt = nil
	reason := "EXPIRED"
	apiKey.RevokedReason = &reason
	apiKey.Revoked = true

	var data APIKeyResourceModel
	mapAPIKeyToModel(apiKey, &data)

	if !data.Username.IsNull() {
		t.Errorf("expected username to be null, got %q", data.Username.ValueString())
	}
	if !data.ExpiresAt.IsNull() {
		t.Errorf("expected expires_at to be null, got %q", data.ExpiresAt.ValueString())
	}
	if !data.Revoked.ValueBool() {
		t.Error("expected revoked true")
	}
	if data.RevokedReason.ValueString() != "EXPIRED" {
		t.Errorf("expected revoked_reason 'EXPIRED', got %q", data.RevokedReason.ValueString())
	}
}
