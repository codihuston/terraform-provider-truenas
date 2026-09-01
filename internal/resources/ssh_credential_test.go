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

const testRemoteHostKey = "ssh-ed25519 AAAAC3Nz\nssh-rsa AAAAB3Nz"

func sshCredentialResource(mock *services.MockKeychainCredentialService) *SSHCredentialResource {
	return &SSHCredentialResource{
		BaseResource: BaseResource{services: &services.TrueNASServices{KeychainCredential: mock}},
	}
}

// testSSHCredential is the connection the API returns for
// testSSHCredentialAttrs.
func testSSHCredential() *services.SSHCredential {
	return &services.SSHCredential{
		ID:             13,
		Name:           "backup-host",
		Host:           "backup.example.com",
		Port:           2222,
		Username:       "truenas_replication",
		PrivateKeyID:   12,
		RemoteHostKey:  testRemoteHostKey,
		ConnectTimeout: 20,
	}
}

// testSSHCredentialAttrs is a settled connection: every attribute is known.
func testSSHCredentialAttrs() map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"id":              tftypes.NewValue(tftypes.String, "13"),
		"name":            tftypes.NewValue(tftypes.String, "backup-host"),
		"host":            tftypes.NewValue(tftypes.String, "backup.example.com"),
		"port":            tftypes.NewValue(tftypes.Number, 2222),
		"username":        tftypes.NewValue(tftypes.String, "truenas_replication"),
		"private_key_id":  tftypes.NewValue(tftypes.String, "12"),
		"remote_host_key": tftypes.NewValue(tftypes.String, testRemoteHostKey),
		"connect_timeout": tftypes.NewValue(tftypes.Number, 20),
	}
}

func assertSSHCredentialState(t *testing.T, data SSHCredentialResourceModel) {
	t.Helper()

	if data.ID.ValueString() != "13" {
		t.Errorf("expected ID '13', got %q", data.ID.ValueString())
	}
	if data.Name.ValueString() != "backup-host" {
		t.Errorf("expected name 'backup-host', got %q", data.Name.ValueString())
	}
	if data.Host.ValueString() != "backup.example.com" {
		t.Errorf("unexpected host: %q", data.Host.ValueString())
	}
	if data.Port.ValueInt64() != 2222 {
		t.Errorf("expected port 2222, got %d", data.Port.ValueInt64())
	}
	if data.Username.ValueString() != "truenas_replication" {
		t.Errorf("unexpected username: %q", data.Username.ValueString())
	}
	// The reference is carried as the string ID Terraform exports, so it can be
	// wired straight to truenas_ssh_keypair.<name>.id.
	if data.PrivateKeyID.ValueString() != "12" {
		t.Errorf("expected private_key_id '12', got %q", data.PrivateKeyID.ValueString())
	}
	if data.RemoteHostKey.ValueString() != testRemoteHostKey {
		t.Errorf("unexpected remote_host_key: %q", data.RemoteHostKey.ValueString())
	}
	if data.ConnectTimeout.ValueInt64() != 20 {
		t.Errorf("expected connect_timeout 20, got %d", data.ConnectTimeout.ValueInt64())
	}
}

func TestNewSSHCredentialResource(t *testing.T) {
	r := NewSSHCredentialResource()
	if r == nil {
		t.Fatal("NewSSHCredentialResource returned nil")
	}

	if _, ok := r.(*SSHCredentialResource); !ok {
		t.Fatalf("expected *SSHCredentialResource, got %T", r)
	}

	_ = resource.Resource(r)
	_ = resource.ResourceWithConfigure(r.(*SSHCredentialResource))
	_ = resource.ResourceWithImportState(r.(*SSHCredentialResource))
}

func TestSSHCredentialResource_Metadata(t *testing.T) {
	resp := &resource.MetadataResponse{}
	NewSSHCredentialResource().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "truenas"}, resp)

	if resp.TypeName != "truenas_ssh_credential" {
		t.Errorf("expected TypeName 'truenas_ssh_credential', got %q", resp.TypeName)
	}
}

func TestSSHCredentialResource_Configure(t *testing.T) {
	r := NewSSHCredentialResource().(*SSHCredentialResource)

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

func TestSSHCredentialResource_Schema(t *testing.T) {
	s := resourceSchema(t, NewSSHCredentialResource())

	if s.Description == "" {
		t.Error("expected non-empty schema description")
	}

	for _, name := range []string{"id", "name", "host", "port", "username", "private_key_id", "remote_host_key", "connect_timeout"} {
		if s.Attributes[name] == nil {
			t.Errorf("expected %q attribute", name)
		}
	}

	for _, name := range []string{"name", "host", "private_key_id"} {
		if !s.Attributes[name].IsRequired() {
			t.Errorf("expected %q to be required", name)
		}
	}
	// remote_host_key is discovered when it is not configured, so it has to be
	// settable as well as computed.
	for _, name := range []string{"port", "username", "remote_host_key", "connect_timeout"} {
		if !s.Attributes[name].IsOptional() || !s.Attributes[name].IsComputed() {
			t.Errorf("expected %q to be optional and computed", name)
		}
	}
	if !s.Attributes["id"].IsComputed() || s.Attributes["id"].IsOptional() {
		t.Error("expected 'id' to be computed-only")
	}
}

// createSSHCredential runs Create against a mock and returns the options the
// service received.
func createSSHCredential(t *testing.T, mock *services.MockKeychainCredentialService, plan map[string]tftypes.Value) *resource.CreateResponse {
	t.Helper()

	r := sshCredentialResource(mock)
	s := resourceSchema(t, r)
	value := objectValue(t, s, plan)

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: value}}, resp)

	return resp
}

// A configured remote_host_key is sent as-is: nothing is scanned, so the remote
// host need not be reachable.
func TestSSHCredentialResource_Create_WithConfiguredHostKey(t *testing.T) {
	var captured services.CreateSSHCredentialOpts
	scanned := false

	resp := createSSHCredential(t, &services.MockKeychainCredentialService{
		CreateSSHCredentialFunc: func(ctx context.Context, opts services.CreateSSHCredentialOpts) (*services.SSHCredential, error) {
			captured = opts
			return testSSHCredential(), nil
		},
		ScanRemoteHostKeyFunc: func(ctx context.Context, opts services.ScanRemoteHostKeyOpts) (string, error) {
			scanned = true
			return "", nil
		},
	}, withAttrs(testSSHCredentialAttrs(), map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}))

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if scanned {
		t.Error("expected no host key scan when remote_host_key is configured")
	}

	if captured.Name != "backup-host" || captured.Host != "backup.example.com" {
		t.Errorf("unexpected connection: %+v", captured)
	}
	if captured.Port != 2222 || captured.ConnectTimeout != 20 {
		t.Errorf("unexpected port or timeout: %+v", captured)
	}
	if captured.Username != "truenas_replication" {
		t.Errorf("unexpected username: %q", captured.Username)
	}
	// The API takes the key pair's integer ID, not the string Terraform holds.
	if captured.PrivateKeyID != 12 {
		t.Errorf("expected private key ID 12, got %d", captured.PrivateKeyID)
	}
	if captured.RemoteHostKey != testRemoteHostKey {
		t.Errorf("unexpected remote host key: %q", captured.RemoteHostKey)
	}

	var data SSHCredentialResourceModel
	resp.State.Get(context.Background(), &data)
	assertSSHCredentialState(t, data)
}

// An unset remote_host_key is discovered by scanning the host once, at create
// time, and trusting whatever answers.
func TestSSHCredentialResource_Create_ScansHostKey(t *testing.T) {
	var captured services.CreateSSHCredentialOpts
	var scanOpts services.ScanRemoteHostKeyOpts

	resp := createSSHCredential(t, &services.MockKeychainCredentialService{
		CreateSSHCredentialFunc: func(ctx context.Context, opts services.CreateSSHCredentialOpts) (*services.SSHCredential, error) {
			captured = opts
			return testSSHCredential(), nil
		},
		ScanRemoteHostKeyFunc: func(ctx context.Context, opts services.ScanRemoteHostKeyOpts) (string, error) {
			scanOpts = opts
			return testRemoteHostKey, nil
		},
	}, withAttrs(testSSHCredentialAttrs(), map[string]tftypes.Value{
		"id":              tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"remote_host_key": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}))

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	// The scan has to use the same address the connection will, or it would
	// pin the key of a different service.
	if scanOpts.Host != "backup.example.com" || scanOpts.Port != 2222 || scanOpts.ConnectTimeout != 20 {
		t.Errorf("unexpected scan options: %+v", scanOpts)
	}
	if captured.RemoteHostKey != testRemoteHostKey {
		t.Errorf("expected the scanned host key to be sent, got %q", captured.RemoteHostKey)
	}
}

// An unreachable host fails the apply rather than storing a connection that
// trusts nothing.
func TestSSHCredentialResource_Create_ScanError(t *testing.T) {
	created := false

	resp := createSSHCredential(t, &services.MockKeychainCredentialService{
		CreateSSHCredentialFunc: func(ctx context.Context, opts services.CreateSSHCredentialOpts) (*services.SSHCredential, error) {
			created = true
			return testSSHCredential(), nil
		},
		ScanRemoteHostKeyFunc: func(ctx context.Context, opts services.ScanRemoteHostKeyOpts) (string, error) {
			return "", errors.New("timed out")
		},
	}, withAttrs(testSSHCredentialAttrs(), map[string]tftypes.Value{
		"remote_host_key": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}))

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error when the host key cannot be scanned")
	}
	if created {
		t.Error("expected no connection to be created when the scan fails")
	}
}

func TestSSHCredentialResource_Create_InvalidPrivateKeyID(t *testing.T) {
	resp := createSSHCredential(t, &services.MockKeychainCredentialService{},
		withAttrs(testSSHCredentialAttrs(), map[string]tftypes.Value{
			"private_key_id": tftypes.NewValue(tftypes.String, "not-a-number"),
		}))

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric private_key_id")
	}
}

func TestSSHCredentialResource_Create_APIError(t *testing.T) {
	resp := createSSHCredential(t, &services.MockKeychainCredentialService{
		CreateSSHCredentialFunc: func(ctx context.Context, opts services.CreateSSHCredentialOpts) (*services.SSHCredential, error) {
			return nil, errors.New("connection refused")
		},
	}, testSSHCredentialAttrs())

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSSHCredentialResource_Create_InvalidPlan(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	bad := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: bad}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable plan")
	}
}

func TestSSHCredentialResource_Read_Success(t *testing.T) {
	var capturedID int64

	r := sshCredentialResource(&services.MockKeychainCredentialService{
		GetSSHCredentialFunc: func(ctx context.Context, id int64) (*services.SSHCredential, error) {
			capturedID = id
			return testSSHCredential(), nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHCredentialAttrs())

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 13 {
		t.Errorf("expected id 13, got %d", capturedID)
	}

	var data SSHCredentialResourceModel
	resp.State.Get(context.Background(), &data)
	assertSSHCredentialState(t, data)
}

// A host key edited on the appliance surfaces as drift, so a configured
// remote_host_key is enforced rather than merely recorded.
func TestSSHCredentialResource_Read_ReportsHostKeyDrift(t *testing.T) {
	drifted := testSSHCredential()
	drifted.RemoteHostKey = "ssh-ed25519 BBBB"

	r := sshCredentialResource(&services.MockKeychainCredentialService{
		GetSSHCredentialFunc: func(ctx context.Context, id int64) (*services.SSHCredential, error) {
			return drifted, nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHCredentialAttrs())

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var data SSHCredentialResourceModel
	resp.State.Get(context.Background(), &data)
	if data.RemoteHostKey.ValueString() != drifted.RemoteHostKey {
		t.Errorf("expected the server's host key to surface as drift, got %q", data.RemoteHostKey.ValueString())
	}
}

func TestSSHCredentialResource_Read_NotFoundRemovesResource(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{
		GetSSHCredentialFunc: func(ctx context.Context, id int64) (*services.SSHCredential, error) {
			return nil, nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHCredentialAttrs())

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected the resource to be removed from state")
	}
}

func TestSSHCredentialResource_Read_APIError(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{
		GetSSHCredentialFunc: func(ctx context.Context, id int64) (*services.SSHCredential, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHCredentialAttrs())

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSSHCredentialResource_Read_InvalidID(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	state := objectValue(t, s, withAttrs(testSSHCredentialAttrs(), map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "not-a-number"),
	}))

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestSSHCredentialResource_Read_InvalidState(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	bad := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: bad}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: bad}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable state")
	}
}

// A settled remote_host_key is resubmitted rather than re-scanned, so an update
// never silently re-pins the host.
func TestSSHCredentialResource_Update_Success(t *testing.T) {
	var capturedID int64
	var captured services.UpdateSSHCredentialOpts
	scanned := false

	updated := testSSHCredential()
	updated.Username = "backup"

	r := sshCredentialResource(&services.MockKeychainCredentialService{
		UpdateSSHCredentialFunc: func(ctx context.Context, id int64, opts services.UpdateSSHCredentialOpts) (*services.SSHCredential, error) {
			capturedID = id
			captured = opts
			return updated, nil
		},
		ScanRemoteHostKeyFunc: func(ctx context.Context, opts services.ScanRemoteHostKeyOpts) (string, error) {
			scanned = true
			return "", nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHCredentialAttrs())
	plan := objectValue(t, s, withAttrs(testSSHCredentialAttrs(), map[string]tftypes.Value{
		"username": tftypes.NewValue(tftypes.String, "backup"),
	}))

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: s, Raw: state},
		Plan:  tfsdk.Plan{Schema: s, Raw: plan},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if scanned {
		t.Error("expected no host key scan on update")
	}
	if capturedID != 13 {
		t.Errorf("expected id 13, got %d", capturedID)
	}
	if captured.Username != "backup" {
		t.Errorf("expected username 'backup', got %q", captured.Username)
	}
	if captured.RemoteHostKey != testRemoteHostKey {
		t.Errorf("expected the stored host key to be resubmitted, got %q", captured.RemoteHostKey)
	}

	var data SSHCredentialResourceModel
	resp.State.Get(context.Background(), &data)
	if data.Username.ValueString() != "backup" {
		t.Errorf("expected username 'backup' in state, got %q", data.Username.ValueString())
	}
}

func TestSSHCredentialResource_Update_APIError(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{
		UpdateSSHCredentialFunc: func(ctx context.Context, id int64, opts services.UpdateSSHCredentialOpts) (*services.SSHCredential, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	value := objectValue(t, s, testSSHCredentialAttrs())

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: s, Raw: value},
		Plan:  tfsdk.Plan{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSSHCredentialResource_Update_InvalidID(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	value := objectValue(t, s, withAttrs(testSSHCredentialAttrs(), map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "not-a-number"),
	}))

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: s, Raw: value},
		Plan:  tfsdk.Plan{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestSSHCredentialResource_Update_InvalidPrivateKeyID(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHCredentialAttrs())
	plan := objectValue(t, s, withAttrs(testSSHCredentialAttrs(), map[string]tftypes.Value{
		"private_key_id": tftypes.NewValue(tftypes.String, "not-a-number"),
	}))

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: s, Raw: state},
		Plan:  tfsdk.Plan{Schema: s, Raw: plan},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric private_key_id")
	}
}

func TestSSHCredentialResource_Update_InvalidPlan(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	bad := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: s, Raw: bad},
		Plan:  tfsdk.Plan{Schema: s, Raw: bad},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable plan")
	}
}

func TestSSHCredentialResource_Delete_Success(t *testing.T) {
	var capturedID int64

	r := sshCredentialResource(&services.MockKeychainCredentialService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			capturedID = id
			return nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHCredentialAttrs())

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 13 {
		t.Errorf("expected id 13, got %d", capturedID)
	}
}

func TestSSHCredentialResource_Delete_APIError(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			return errors.New("This credential is used and no cascade option is specified")
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, testSSHCredentialAttrs())

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API error")
	}
}

func TestSSHCredentialResource_Delete_InvalidID(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	state := objectValue(t, s, withAttrs(testSSHCredentialAttrs(), map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "not-a-number"),
	}))

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestSSHCredentialResource_Delete_InvalidState(t *testing.T) {
	r := sshCredentialResource(&services.MockKeychainCredentialService{})

	s := resourceSchema(t, r)
	bad := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: bad}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: bad}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for undecodable state")
	}
}
