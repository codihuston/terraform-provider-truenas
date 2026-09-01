package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const testPrivateKey = "-----BEGIN OPENSSH PRIVATE KEY-----\nsecret\n-----END OPENSSH PRIVATE KEY-----\n"

func sshKeypairResource(mock *services.MockKeychainCredentialService) *SSHKeypairResource {
	return &SSHKeypairResource{
		BaseResource: BaseResource{services: &services.TrueNASServices{KeychainCredential: mock}},
	}
}

// testSSHKeyPair is the key pair the API returns for testSSHKeypairAttrs.
func testSSHKeyPair() *services.SSHKeyPair {
	return &services.SSHKeyPair{
		ID:        12,
		Name:      "backup-host",
		PublicKey: "ssh-ed25519 AAAAC3Nz backup-host\n",
	}
}

// testSSHKeypairAttrs is a settled key pair: everything the API reports is in
// state, and private_key is absent because it is write-only.
func testSSHKeypairAttrs() map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"id":                     tftypes.NewValue(tftypes.String, "12"),
		"name":                   tftypes.NewValue(tftypes.String, "backup-host"),
		"private_key_wo_version": tftypes.NewValue(tftypes.Number, 1),
		"public_key":             tftypes.NewValue(tftypes.String, "ssh-ed25519 AAAAC3Nz backup-host\n"),
	}
}

func assertSSHKeypairState(t *testing.T, data SSHKeypairResourceModel) {
	t.Helper()

	if data.ID.ValueString() != "12" {
		t.Errorf("expected ID '12', got %q", data.ID.ValueString())
	}
	if data.Name.ValueString() != "backup-host" {
		t.Errorf("expected name 'backup-host', got %q", data.Name.ValueString())
	}
	if data.PublicKey.ValueString() != "ssh-ed25519 AAAAC3Nz backup-host\n" {
		t.Errorf("unexpected public_key: %q", data.PublicKey.ValueString())
	}
	if !data.PrivateKey.IsNull() {
		t.Errorf("expected private_key to stay out of state, got %q", data.PrivateKey.ValueString())
	}
}

func TestNewSSHKeypairResource(t *testing.T) {
	r := NewSSHKeypairResource()
	if r == nil {
		t.Fatal("NewSSHKeypairResource returned nil")
	}

	if _, ok := r.(*SSHKeypairResource); !ok {
		t.Fatalf("expected *SSHKeypairResource, got %T", r)
	}

	_ = resource.Resource(r)
	_ = resource.ResourceWithConfigure(r.(*SSHKeypairResource))
	_ = resource.ResourceWithImportState(r.(*SSHKeypairResource))
}

func TestSSHKeypairResource_Metadata(t *testing.T) {
	resp := &resource.MetadataResponse{}
	NewSSHKeypairResource().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "truenas"}, resp)

	if resp.TypeName != "truenas_ssh_keypair" {
		t.Errorf("expected TypeName 'truenas_ssh_keypair', got %q", resp.TypeName)
	}
}

func TestSSHKeypairResource_Configure(t *testing.T) {
	r := NewSSHKeypairResource().(*SSHKeypairResource)

	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &services.TrueNASServices{}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	resp = &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	resp = &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "not a client"}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for wrong ProviderData type")
	}
}

func TestSSHKeypairResource_Schema(t *testing.T) {
	s := resourceSchema(t, NewSSHKeypairResource())

	if s.Description == "" {
		t.Error("expected non-empty schema description")
	}

	for _, name := range []string{"id", "name", "private_key", "private_key_wo_version", "public_key"} {
		if s.Attributes[name] == nil {
			t.Errorf("expected %q attribute", name)
		}
	}

	// The private key never reaches state: it is write-only and TrueNAS is not
	// asked for it, so state cannot leak it and it cannot produce drift.
	privateKey, ok := s.Attributes["private_key"].(rschema.StringAttribute)
	if !ok {
		t.Fatalf("expected private_key to be a string attribute, got %T", s.Attributes["private_key"])
	}
	if !privateKey.WriteOnly || !privateKey.Sensitive || !privateKey.Required {
		t.Error("expected private_key to be a required, sensitive, write-only attribute")
	}

	// public_key is derived by TrueNAS, so making it settable would guarantee
	// an "inconsistent result after apply" error.
	if !s.Attributes["public_key"].IsComputed() || s.Attributes["public_key"].IsOptional() {
		t.Error("expected 'public_key' to be computed-only")
	}
	if !s.Attributes["private_key_wo_version"].IsRequired() {
		t.Error("expected 'private_key_wo_version' to be required")
	}
}

// An empty private key is rejected during plan rather than on apply, matching
// the API's own requirement.
func TestSSHKeypairResource_Schema_RejectsEmptyPrivateKey(t *testing.T) {
	s := resourceSchema(t, NewSSHKeypairResource())

	privateKey := s.Attributes["private_key"].(rschema.StringAttribute)
	if len(privateKey.Validators) == 0 {
		t.Fatal("expected private_key to be validated")
	}
}

func TestSSHKeypairResource_Create_Success(t *testing.T) {
	var captured services.CreateSSHKeyPairOpts

	r := sshKeypairResource(&services.MockKeychainCredentialService{
		CreateSSHKeyPairFunc: func(ctx context.Context, opts services.CreateSSHKeyPairOpts) (*services.SSHKeyPair, error) {
			captured = opts
			return testSSHKeyPair(), nil
		},
	})

	s := resourceSchema(t, r)
	plan := objectValue(t, s, withAttrs(testSSHKeypairAttrs(), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"public_key": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}))
	config := objectValue(t, s, withAttrs(testSSHKeypairAttrs(), map[string]tftypes.Value{
		"id":          tftypes.NewValue(tftypes.String, nil),
		"public_key":  tftypes.NewValue(tftypes.String, nil),
		"private_key": tftypes.NewValue(tftypes.String, testPrivateKey),
	}))

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{
		Plan:   tfsdk.Plan{Schema: s, Raw: plan},
		Config: tfsdk.Config{Schema: s, Raw: config},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if captured.Name != "backup-host" {
		t.Errorf("expected name 'backup-host', got %q", captured.Name)
	}
	// The key comes from the configuration: a write-only attribute is null in
	// the plan.
	if captured.PrivateKey != testPrivateKey {
		t.Errorf("expected the private key from config to be sent, got %q", captured.PrivateKey)
	}

	var data SSHKeypairResourceModel
	resp.State.Get(context.Background(), &data)
	assertSSHKeypairState(t, data)
}

func TestSSHKeypairResource_Create_APIError(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{
		CreateSSHKeyPairFunc: func(ctx context.Context, opts services.CreateSSHKeyPairOpts) (*services.SSHKeyPair, error) {
			return nil, errors.New("error in libcrypto")
		},
	})

	s := resourceSchema(t, r)
	value := objectValue(t, s, testSSHKeypairAttrs())

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{
		Plan:   tfsdk.Plan{Schema: s, Raw: value},
		Config: tfsdk.Config{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSSHKeypairResource_Create_InvalidPlan(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	bad := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{
		Plan:   tfsdk.Plan{Schema: s, Raw: bad},
		Config: tfsdk.Config{Schema: s, Raw: bad},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable plan")
	}
}

func TestSSHKeypairResource_Read_Success(t *testing.T) {
	var capturedID int64

	r := sshKeypairResource(&services.MockKeychainCredentialService{
		GetSSHKeyPairFunc: func(ctx context.Context, id int64) (*services.SSHKeyPair, error) {
			capturedID = id
			return testSSHKeyPair(), nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHKeypairAttrs())

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 12 {
		t.Errorf("expected id 12, got %d", capturedID)
	}

	var data SSHKeypairResourceModel
	resp.State.Get(context.Background(), &data)
	assertSSHKeypairState(t, data)
}

// A public key replaced outside Terraform surfaces as drift; the private key
// behind it cannot, because the provider never reads it.
func TestSSHKeypairResource_Read_ReportsPublicKeyDrift(t *testing.T) {
	drifted := testSSHKeyPair()
	drifted.PublicKey = "ssh-ed25519 BBBB intruder@example\n"

	r := sshKeypairResource(&services.MockKeychainCredentialService{
		GetSSHKeyPairFunc: func(ctx context.Context, id int64) (*services.SSHKeyPair, error) {
			return drifted, nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHKeypairAttrs())

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var data SSHKeypairResourceModel
	resp.State.Get(context.Background(), &data)
	if data.PublicKey.ValueString() != drifted.PublicKey {
		t.Errorf("expected the server's public key to surface as drift, got %q", data.PublicKey.ValueString())
	}
}

func TestSSHKeypairResource_Read_NotFoundRemovesResource(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{
		GetSSHKeyPairFunc: func(ctx context.Context, id int64) (*services.SSHKeyPair, error) {
			return nil, nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHKeypairAttrs())

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected the resource to be removed from state")
	}
}

func TestSSHKeypairResource_Read_APIError(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{
		GetSSHKeyPairFunc: func(ctx context.Context, id int64) (*services.SSHKeyPair, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHKeypairAttrs())

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSSHKeypairResource_Read_InvalidID(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	state := objectValue(t, s, withAttrs(testSSHKeypairAttrs(), map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "not-a-number"),
	}))

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestSSHKeypairResource_Read_InvalidState(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	bad := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: bad}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: bad}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable state")
	}
}

// updateSSHKeypair runs Update and returns the options the service received.
func updateSSHKeypair(t *testing.T, state, plan, config map[string]tftypes.Value) (services.UpdateSSHKeyPairOpts, *resource.UpdateResponse) {
	t.Helper()

	var captured services.UpdateSSHKeyPairOpts
	r := sshKeypairResource(&services.MockKeychainCredentialService{
		UpdateSSHKeyPairFunc: func(ctx context.Context, id int64, opts services.UpdateSSHKeyPairOpts) (*services.SSHKeyPair, error) {
			captured = opts
			return testSSHKeyPair(), nil
		},
	})

	s := resourceSchema(t, r)
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(context.Background(), resource.UpdateRequest{
		State:  tfsdk.State{Schema: s, Raw: objectValue(t, s, state)},
		Plan:   tfsdk.Plan{Schema: s, Raw: objectValue(t, s, plan)},
		Config: tfsdk.Config{Schema: s, Raw: objectValue(t, s, config)},
	}, resp)

	return captured, resp
}

// The stored key is only replaced when private_key_wo_version changes: re-sending
// on every apply would rotate the key behind the remote authorized_keys.
func TestSSHKeypairResource_Update_PrivateKeyFollowsVersion(t *testing.T) {
	settled := testSSHKeypairAttrs()
	withKey := withAttrs(settled, map[string]tftypes.Value{
		"private_key": tftypes.NewValue(tftypes.String, testPrivateKey),
	})

	renamed := withAttrs(settled, map[string]tftypes.Value{
		"name": tftypes.NewValue(tftypes.String, "renamed"),
	})

	unchanged, resp := updateSSHKeypair(t, settled, renamed, withAttrs(withKey, map[string]tftypes.Value{
		"name": tftypes.NewValue(tftypes.String, "renamed"),
	}))
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if unchanged.Name != "renamed" {
		t.Errorf("expected name 'renamed', got %q", unchanged.Name)
	}
	if unchanged.PrivateKey != nil {
		t.Error("expected the private key to be withheld while private_key_wo_version is unchanged")
	}

	bumped := withAttrs(settled, map[string]tftypes.Value{
		"private_key_wo_version": tftypes.NewValue(tftypes.Number, 2),
	})
	bumpedConfig := withAttrs(withKey, map[string]tftypes.Value{
		"private_key_wo_version": tftypes.NewValue(tftypes.Number, 2),
	})

	rotated, resp := updateSSHKeypair(t, settled, bumped, bumpedConfig)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if rotated.PrivateKey == nil || *rotated.PrivateKey != testPrivateKey {
		t.Errorf("expected the private key to be sent when private_key_wo_version changes, got %v", rotated.PrivateKey)
	}
}

func TestSSHKeypairResource_Update_Success(t *testing.T) {
	settled := testSSHKeypairAttrs()

	_, resp := updateSSHKeypair(t, settled, settled, settled)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var data SSHKeypairResourceModel
	resp.State.Get(context.Background(), &data)
	assertSSHKeypairState(t, data)
}

func TestSSHKeypairResource_Update_APIError(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{
		UpdateSSHKeyPairFunc: func(ctx context.Context, id int64, opts services.UpdateSSHKeyPairOpts) (*services.SSHKeyPair, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	value := objectValue(t, s, testSSHKeypairAttrs())

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(context.Background(), resource.UpdateRequest{
		State:  tfsdk.State{Schema: s, Raw: value},
		Plan:   tfsdk.Plan{Schema: s, Raw: value},
		Config: tfsdk.Config{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSSHKeypairResource_Update_InvalidID(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	value := objectValue(t, s, withAttrs(testSSHKeypairAttrs(), map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "not-a-number"),
	}))

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(context.Background(), resource.UpdateRequest{
		State:  tfsdk.State{Schema: s, Raw: value},
		Plan:   tfsdk.Plan{Schema: s, Raw: value},
		Config: tfsdk.Config{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestSSHKeypairResource_Update_InvalidPlan(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	bad := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(context.Background(), resource.UpdateRequest{
		State:  tfsdk.State{Schema: s, Raw: bad},
		Plan:   tfsdk.Plan{Schema: s, Raw: bad},
		Config: tfsdk.Config{Schema: s, Raw: bad},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable plan")
	}
}

func TestSSHKeypairResource_Delete_Success(t *testing.T) {
	var capturedID int64

	r := sshKeypairResource(&services.MockKeychainCredentialService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			capturedID = id
			return nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHKeypairAttrs())

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 12 {
		t.Errorf("expected id 12, got %d", capturedID)
	}
}

// TrueNAS refuses to delete a key pair something still references, and the
// provider surfaces that rather than forcing the deletion through.
func TestSSHKeypairResource_Delete_APIError(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			return errors.New("This credential is used and no cascade option is specified")
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHKeypairAttrs())

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSSHKeypairResource_Delete_InvalidID(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	state := objectValue(t, s, withAttrs(testSSHKeypairAttrs(), map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "not-a-number"),
	}))

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestSSHKeypairResource_Delete_InvalidState(t *testing.T) {
	r := sshKeypairResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	bad := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: bad}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: bad}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable state")
	}
}
