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

func testUser() *services.User {
	return &services.User{
		ID:                   71,
		UID:                  3000,
		Username:             "deploy",
		FullName:             "Deployment Account",
		Home:                 "/mnt/tank/home/deploy",
		Shell:                "/usr/bin/bash",
		Group:                110,
		GroupGID:             3000,
		Groups:               []int64{91},
		PasswordDisabled:     true,
		SSHPublicKey:         "ssh-ed25519 AAAA deploy@example",
		SudoCommands:         []string{},
		SudoCommandsNoPasswd: []string{"/usr/bin/systemctl restart app"},
	}
}

// userResource builds a user resource backed by the supplied mock service.
func userResource(mock *services.MockUserService) *UserResource {
	return &UserResource{
		BaseResource: BaseResource{services: &services.TrueNASServices{User: mock}},
	}
}

// keyOnlyUserAttrs is the attribute set of a passwordless, key-authenticated
// account, as produced by the defaults in the schema.
func keyOnlyUserAttrs() map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"username":               tftypes.NewValue(tftypes.String, "deploy"),
		"full_name":              tftypes.NewValue(tftypes.String, "Deployment Account"),
		"uid":                    tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
		"group":                  tftypes.NewValue(tftypes.Number, nil),
		"group_create":           tftypes.NewValue(tftypes.Bool, true),
		"groups":                 tftypes.NewValue(tftypes.Set{ElementType: tftypes.Number}, tftypes.UnknownValue),
		"home":                   tftypes.NewValue(tftypes.String, "/mnt/tank/home"),
		"home_create":            tftypes.NewValue(tftypes.Bool, true),
		"home_mode":              tftypes.NewValue(tftypes.String, "700"),
		"shell":                  tftypes.NewValue(tftypes.String, "/usr/bin/bash"),
		"ssh_public_key":         tftypes.NewValue(tftypes.String, "ssh-ed25519 AAAA deploy@example"),
		"ssh_password_enabled":   tftypes.NewValue(tftypes.Bool, false),
		"smb":                    tftypes.NewValue(tftypes.Bool, false),
		"locked":                 tftypes.NewValue(tftypes.Bool, false),
		"password_disabled":      tftypes.NewValue(tftypes.Bool, true),
		"sudo_commands":          stringList(),
		"sudo_commands_nopasswd": stringList("/usr/bin/systemctl restart app"),
		"delete_group":           tftypes.NewValue(tftypes.Bool, true),
	}
}

// withAttrs returns a copy of attrs with the overrides applied.
func withAttrs(attrs map[string]tftypes.Value, overrides map[string]tftypes.Value) map[string]tftypes.Value {
	merged := make(map[string]tftypes.Value, len(attrs)+len(overrides))
	for name, value := range attrs {
		merged[name] = value
	}
	for name, value := range overrides {
		merged[name] = value
	}
	return merged
}

func TestNewUserResource(t *testing.T) {
	r := NewUserResource()

	u, ok := r.(*UserResource)
	if !ok {
		t.Fatalf("expected *UserResource, got %T", r)
	}

	_ = resource.ResourceWithConfigure(u)
	_ = resource.ResourceWithImportState(u)
	_ = resource.ResourceWithValidateConfig(u)
}

func TestUserResource_Metadata(t *testing.T) {
	resp := &resource.MetadataResponse{}
	NewUserResource().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "truenas"}, resp)

	if resp.TypeName != "truenas_user" {
		t.Errorf("expected TypeName 'truenas_user', got %q", resp.TypeName)
	}
}

func TestUserResource_Schema(t *testing.T) {
	s := resourceSchema(t, NewUserResource())

	if s.Description == "" {
		t.Error("expected non-empty schema description")
	}

	for _, name := range []string{
		"id", "username", "full_name", "uid", "group", "group_create", "groups",
		"home", "home_create", "home_mode", "home_path", "shell", "email",
		"ssh_public_key", "ssh_password_enabled", "smb", "locked",
		"password_disabled", "password", "password_wo_version",
		"sudo_commands", "sudo_commands_nopasswd", "delete_group", "builtin",
	} {
		if s.Attributes[name] == nil {
			t.Errorf("expected %q attribute", name)
		}
	}

	if !s.Attributes["password"].IsWriteOnly() {
		t.Error("expected 'password' to be write-only so it never reaches state")
	}
	if !s.Attributes["password"].IsSensitive() {
		t.Error("expected 'password' to be sensitive")
	}
}

func TestUserResource_ValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]tftypes.Value
		wantError bool
	}{
		{
			name:      "group created",
			overrides: nil,
		},
		{
			name: "existing group",
			overrides: map[string]tftypes.Value{
				"group":        tftypes.NewValue(tftypes.Number, 110),
				"group_create": tftypes.NewValue(tftypes.Bool, nil),
			},
		},
		{
			name: "unresolved group reference",
			overrides: map[string]tftypes.Value{
				"group":        tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
				"group_create": tftypes.NewValue(tftypes.Bool, nil),
			},
		},
		{
			name: "no primary group",
			overrides: map[string]tftypes.Value{
				"group":        tftypes.NewValue(tftypes.Number, nil),
				"group_create": tftypes.NewValue(tftypes.Bool, nil),
			},
			wantError: true,
		},
		{
			name: "both primary group options",
			overrides: map[string]tftypes.Value{
				"group": tftypes.NewValue(tftypes.Number, 110),
			},
			wantError: true,
		},
		{
			name: "password paired with its version",
			overrides: map[string]tftypes.Value{
				"password":            tftypes.NewValue(tftypes.String, "hunter2"),
				"password_wo_version": tftypes.NewValue(tftypes.Number, 1),
				"password_disabled":   tftypes.NewValue(tftypes.Bool, false),
			},
		},
		{
			name: "password without a version",
			overrides: map[string]tftypes.Value{
				"password":          tftypes.NewValue(tftypes.String, "hunter2"),
				"password_disabled": tftypes.NewValue(tftypes.Bool, false),
			},
			wantError: true,
		},
		{
			name: "version without a password",
			overrides: map[string]tftypes.Value{
				"password_wo_version": tftypes.NewValue(tftypes.Number, 2),
			},
			wantError: true,
		},
		{
			name: "smb with password authentication disabled",
			overrides: map[string]tftypes.Value{
				"smb": tftypes.NewValue(tftypes.Bool, true),
			},
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := NewUserResource().(*UserResource)
			s := resourceSchema(t, r)
			config := objectValue(t, s, withAttrs(keyOnlyUserAttrs(), tc.overrides))

			resp := &resource.ValidateConfigResponse{}
			r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{
				Config: tfsdk.Config{Schema: s, Raw: config},
			}, resp)

			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Fatalf("expected error %v, got %v (%v)", tc.wantError, got, resp.Diagnostics)
			}
		})
	}
}

func TestUserResource_Create_KeyOnly(t *testing.T) {
	var captured services.CreateUserOpts

	r := userResource(&services.MockUserService{
		CreateFunc: func(ctx context.Context, opts services.CreateUserOpts) (*services.User, error) {
			captured = opts
			return testUser(), nil
		},
	})

	s := resourceSchema(t, r)
	// group_create leaves the primary group unresolved until apply.
	plan := objectValue(t, s, withAttrs(keyOnlyUserAttrs(), map[string]tftypes.Value{
		"group": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
	}))
	config := objectValue(t, s, keyOnlyUserAttrs())

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{
		Plan:   tfsdk.Plan{Schema: s, Raw: plan},
		Config: tfsdk.Config{Schema: s, Raw: config},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if captured.Username != "deploy" {
		t.Errorf("expected username 'deploy', got %q", captured.Username)
	}
	if !captured.GroupCreate {
		t.Error("expected group_create true")
	}
	if captured.Group != nil {
		t.Errorf("expected group to be omitted, got %d", *captured.Group)
	}
	if captured.UID != nil {
		t.Errorf("expected uid to be omitted, got %d", *captured.UID)
	}
	if !captured.PasswordDisabled {
		t.Error("expected password_disabled true")
	}
	if captured.Password != nil {
		t.Error("expected no password to be sent")
	}
	if !captured.HomeCreate {
		t.Error("expected home_create true")
	}
	if captured.Home != "/mnt/tank/home" {
		t.Errorf("expected home '/mnt/tank/home', got %q", captured.Home)
	}
	if captured.SSHPublicKey == nil || *captured.SSHPublicKey != "ssh-ed25519 AAAA deploy@example" {
		t.Errorf("expected the SSH public key to be sent, got %v", captured.SSHPublicKey)
	}
	if captured.Email != nil {
		t.Errorf("expected email to be omitted, got %q", *captured.Email)
	}

	var state UserResourceModel
	resp.State.Get(context.Background(), &state)
	if state.ID.ValueString() != "71" {
		t.Errorf("expected ID '71', got %q", state.ID.ValueString())
	}
	if state.UID.ValueInt64() != 3000 {
		t.Errorf("expected uid 3000, got %d", state.UID.ValueInt64())
	}
	if state.Group.ValueInt64() != 110 {
		t.Errorf("expected group 110, got %d", state.Group.ValueInt64())
	}
	// home stays as configured; the server reports the created path separately.
	if state.Home.ValueString() != "/mnt/tank/home" {
		t.Errorf("expected home '/mnt/tank/home', got %q", state.Home.ValueString())
	}
	if state.HomePath.ValueString() != "/mnt/tank/home/deploy" {
		t.Errorf("expected home_path '/mnt/tank/home/deploy', got %q", state.HomePath.ValueString())
	}
	if !state.Password.IsNull() {
		t.Error("expected password to stay out of state")
	}
	if !state.Email.IsNull() {
		t.Error("expected an unset email to stay null")
	}
}

func TestUserResource_Create_WithPasswordAndExistingGroup(t *testing.T) {
	var captured services.CreateUserOpts

	r := userResource(&services.MockUserService{
		CreateFunc: func(ctx context.Context, opts services.CreateUserOpts) (*services.User, error) {
			captured = opts
			return testUser(), nil
		},
	})

	s := resourceSchema(t, r)
	attrs := withAttrs(keyOnlyUserAttrs(), map[string]tftypes.Value{
		"uid":               tftypes.NewValue(tftypes.Number, 4200),
		"group":             tftypes.NewValue(tftypes.Number, 110),
		"group_create":      tftypes.NewValue(tftypes.Bool, false),
		"groups":            int64Set(91, 92),
		"password_disabled": tftypes.NewValue(tftypes.Bool, false),
		"email":             tftypes.NewValue(tftypes.String, "deploy@example.com"),
	})
	plan := objectValue(t, s, attrs)
	config := objectValue(t, s, withAttrs(attrs, map[string]tftypes.Value{
		"password": tftypes.NewValue(tftypes.String, "correct horse battery staple"),
	}))

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{
		Plan:   tfsdk.Plan{Schema: s, Raw: plan},
		Config: tfsdk.Config{Schema: s, Raw: config},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if captured.UID == nil || *captured.UID != 4200 {
		t.Errorf("expected uid 4200, got %v", captured.UID)
	}
	if captured.Group == nil || *captured.Group != 110 {
		t.Errorf("expected group 110, got %v", captured.Group)
	}
	if captured.GroupCreate {
		t.Error("expected group_create false")
	}
	if len(captured.Groups) != 2 {
		t.Errorf("expected two additional groups, got %v", captured.Groups)
	}
	if captured.Password == nil || *captured.Password != "correct horse battery staple" {
		t.Errorf("expected the password from config to be sent, got %v", captured.Password)
	}
	if captured.Email == nil || *captured.Email != "deploy@example.com" {
		t.Errorf("expected email to be sent, got %v", captured.Email)
	}
}

func TestUserResource_Create_APIError(t *testing.T) {
	r := userResource(&services.MockUserService{
		CreateFunc: func(ctx context.Context, opts services.CreateUserOpts) (*services.User, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	value := objectValue(t, s, keyOnlyUserAttrs())

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{
		Plan:   tfsdk.Plan{Schema: s, Raw: value},
		Config: tfsdk.Config{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API failure")
	}
}

func TestUserResource_Read_Success(t *testing.T) {
	r := userResource(&services.MockUserService{
		GetFunc: func(ctx context.Context, id int64) (*services.User, error) {
			if id != 71 {
				t.Errorf("expected id 71, got %d", id)
			}
			return testUser(), nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "71"),
	})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var result UserResourceModel
	resp.State.Get(context.Background(), &result)
	if result.Username.ValueString() != "deploy" {
		t.Errorf("expected username 'deploy', got %q", result.Username.ValueString())
	}
	if result.HomePath.ValueString() != "/mnt/tank/home/deploy" {
		t.Errorf("expected home_path '/mnt/tank/home/deploy', got %q", result.HomePath.ValueString())
	}
	if !result.Password.IsNull() {
		t.Error("expected password to stay out of state")
	}
}

func TestUserResource_Read_NotFound(t *testing.T) {
	r := userResource(&services.MockUserService{
		GetFunc: func(ctx context.Context, id int64) (*services.User, error) {
			return nil, nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "71"),
	})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected resource to be removed from state")
	}
}

func TestUserResource_Read_APIError(t *testing.T) {
	r := userResource(&services.MockUserService{
		GetFunc: func(ctx context.Context, id int64) (*services.User, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "71"),
	})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API failure")
	}
}

func TestUserResource_Read_InvalidID(t *testing.T) {
	r := userResource(&services.MockUserService{})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "not-a-number"),
	})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unparsable ID")
	}
}

// updateUser runs an update against a mock and returns the options it received.
func updateUser(t *testing.T, stateAttrs, planAttrs, configAttrs map[string]tftypes.Value) (services.UpdateUserOpts, *resource.UpdateResponse) {
	t.Helper()

	var captured services.UpdateUserOpts
	r := userResource(&services.MockUserService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateUserOpts) (*services.User, error) {
			captured = opts
			return testUser(), nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, stateAttrs)
	plan := objectValue(t, s, planAttrs)
	config := objectValue(t, s, configAttrs)

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Update(context.Background(), resource.UpdateRequest{
		State:  tfsdk.State{Schema: s, Raw: state},
		Plan:   tfsdk.Plan{Schema: s, Raw: plan},
		Config: tfsdk.Config{Schema: s, Raw: config},
	}, resp)

	return captured, resp
}

func TestUserResource_Update_HomeFollowsTheServerUntilHomeChanges(t *testing.T) {
	settled := withAttrs(keyOnlyUserAttrs(), map[string]tftypes.Value{
		"id":        tftypes.NewValue(tftypes.String, "71"),
		"uid":       tftypes.NewValue(tftypes.Number, 3000),
		"group":     tftypes.NewValue(tftypes.Number, 110),
		"groups":    int64Set(91),
		"home_path": tftypes.NewValue(tftypes.String, "/mnt/tank/home/deploy"),
	})

	// home is the parent directory of a created home, which TrueNAS rejects on
	// update, so an unchanged home is sent as the path the server assigned.
	unchanged, resp := updateUser(t, settled, settled, settled)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if unchanged.HomeCreate {
		t.Error("expected home_create to be withheld when home is unchanged")
	}
	if unchanged.Home != "/mnt/tank/home/deploy" {
		t.Errorf("expected the assigned home path to be sent, got %q", unchanged.Home)
	}

	moved := withAttrs(settled, map[string]tftypes.Value{
		"home": tftypes.NewValue(tftypes.String, "/mnt/tank/staff"),
	})
	relocated, resp := updateUser(t, settled, moved, moved)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if !relocated.HomeCreate {
		t.Error("expected home_create to be sent when home changes")
	}
	if relocated.Home != "/mnt/tank/staff" {
		t.Errorf("expected home '/mnt/tank/staff', got %q", relocated.Home)
	}
}

func TestUserResource_Update_SSHPublicKeyWhitespaceIsNotDrift(t *testing.T) {
	settled := withAttrs(keyOnlyUserAttrs(), map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, "71"),
		"uid":    tftypes.NewValue(tftypes.Number, 3000),
		"group":  tftypes.NewValue(tftypes.Number, 110),
		"groups": int64Set(91),
		// file() supplies a trailing newline that TrueNAS strips on the way in.
		"ssh_public_key": tftypes.NewValue(tftypes.String, "ssh-ed25519 AAAA deploy@example\n"),
	})

	_, resp := updateUser(t, settled, settled, settled)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var result UserResourceModel
	resp.State.Get(context.Background(), &result)
	if result.SSHPublicKey.ValueString() != "ssh-ed25519 AAAA deploy@example\n" {
		t.Errorf("expected the configured key to be preserved, got %q", result.SSHPublicKey.ValueString())
	}
}

func TestUserResource_Update_SSHPublicKeyIsRevocable(t *testing.T) {
	settled := withAttrs(keyOnlyUserAttrs(), map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, "71"),
		"uid":    tftypes.NewValue(tftypes.Number, 3000),
		"group":  tftypes.NewValue(tftypes.Number, 110),
		"groups": int64Set(91),
	})
	cleared := withAttrs(settled, map[string]tftypes.Value{
		"ssh_public_key": tftypes.NewValue(tftypes.String, nil),
	})

	captured, resp := updateUser(t, settled, cleared, cleared)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if captured.SSHPublicKey != nil {
		t.Errorf("expected the key to be revoked, got %q", *captured.SSHPublicKey)
	}
}

func TestUserResource_Read_SSHPublicKeyDrift(t *testing.T) {
	r := userResource(&services.MockUserService{
		GetFunc: func(ctx context.Context, id int64) (*services.User, error) {
			user := testUser()
			user.SSHPublicKey = "ssh-ed25519 BBBB intruder@example"
			return user, nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, "71"),
		"ssh_public_key": tftypes.NewValue(tftypes.String, "ssh-ed25519 AAAA deploy@example"),
	})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var result UserResourceModel
	resp.State.Get(context.Background(), &result)
	if result.SSHPublicKey.ValueString() != "ssh-ed25519 BBBB intruder@example" {
		t.Errorf("expected the server's key to surface as drift, got %q", result.SSHPublicKey.ValueString())
	}
}

func TestUserResource_Update_PasswordFollowsVersion(t *testing.T) {
	settled := withAttrs(keyOnlyUserAttrs(), map[string]tftypes.Value{
		"id":                  tftypes.NewValue(tftypes.String, "71"),
		"uid":                 tftypes.NewValue(tftypes.Number, 3000),
		"group":               tftypes.NewValue(tftypes.Number, 110),
		"groups":              int64Set(91),
		"password_disabled":   tftypes.NewValue(tftypes.Bool, false),
		"password_wo_version": tftypes.NewValue(tftypes.Number, 1),
	})
	withPassword := withAttrs(settled, map[string]tftypes.Value{
		"password": tftypes.NewValue(tftypes.String, "correct horse battery staple"),
	})

	unchanged, resp := updateUser(t, settled, settled, withPassword)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if unchanged.Password != nil {
		t.Error("expected the password to be withheld while password_wo_version is unchanged")
	}

	bumped := withAttrs(settled, map[string]tftypes.Value{
		"password_wo_version": tftypes.NewValue(tftypes.Number, 2),
	})
	bumpedConfig := withAttrs(withPassword, map[string]tftypes.Value{
		"password_wo_version": tftypes.NewValue(tftypes.Number, 2),
	})

	rotated, resp := updateUser(t, settled, bumped, bumpedConfig)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if rotated.Password == nil || *rotated.Password != "correct horse battery staple" {
		t.Errorf("expected the password to be sent when password_wo_version changes, got %v", rotated.Password)
	}
}

func TestUserResource_Update_Success(t *testing.T) {
	settled := withAttrs(keyOnlyUserAttrs(), map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, "71"),
		"uid":    tftypes.NewValue(tftypes.Number, 3000),
		"group":  tftypes.NewValue(tftypes.Number, 110),
		"groups": int64Set(91),
	})
	renamed := withAttrs(settled, map[string]tftypes.Value{
		"full_name": tftypes.NewValue(tftypes.String, "Deploy Bot"),
		"locked":    tftypes.NewValue(tftypes.Bool, true),
	})

	captured, resp := updateUser(t, settled, renamed, renamed)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if captured.FullName != "Deploy Bot" {
		t.Errorf("expected full_name 'Deploy Bot', got %q", captured.FullName)
	}
	if !captured.Locked {
		t.Error("expected locked true")
	}
	if captured.Group == nil || *captured.Group != 110 {
		t.Errorf("expected group 110, got %v", captured.Group)
	}

	var state UserResourceModel
	resp.State.Get(context.Background(), &state)
	if state.ID.ValueString() != "71" {
		t.Errorf("expected ID '71', got %q", state.ID.ValueString())
	}
}

func TestUserResource_Update_APIError(t *testing.T) {
	r := userResource(&services.MockUserService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateUserOpts) (*services.User, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	value := objectValue(t, s, withAttrs(keyOnlyUserAttrs(), map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, "71"),
		"uid":    tftypes.NewValue(tftypes.Number, 3000),
		"group":  tftypes.NewValue(tftypes.Number, 110),
		"groups": int64Set(91),
	}))

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: value}}
	r.Update(context.Background(), resource.UpdateRequest{
		State:  tfsdk.State{Schema: s, Raw: value},
		Plan:   tfsdk.Plan{Schema: s, Raw: value},
		Config: tfsdk.Config{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API failure")
	}
}

func TestUserResource_Update_InvalidID(t *testing.T) {
	r := userResource(&services.MockUserService{})

	s := resourceSchema(t, r)
	value := objectValue(t, s, withAttrs(keyOnlyUserAttrs(), map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, "not-a-number"),
		"uid":    tftypes.NewValue(tftypes.Number, 3000),
		"group":  tftypes.NewValue(tftypes.Number, 110),
		"groups": int64Set(91),
	}))

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: value}}
	r.Update(context.Background(), resource.UpdateRequest{
		State:  tfsdk.State{Schema: s, Raw: value},
		Plan:   tfsdk.Plan{Schema: s, Raw: value},
		Config: tfsdk.Config{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unparsable ID")
	}
}

func TestUserResource_Delete_Success(t *testing.T) {
	var capturedID int64
	var capturedDeleteGroup bool

	r := userResource(&services.MockUserService{
		DeleteFunc: func(ctx context.Context, id int64, deleteGroup bool) error {
			capturedID = id
			capturedDeleteGroup = deleteGroup
			return nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, "71"),
		"delete_group": tftypes.NewValue(tftypes.Bool, false),
	})

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 71 {
		t.Errorf("expected id 71, got %d", capturedID)
	}
	if capturedDeleteGroup {
		t.Error("expected delete_group false to be passed through")
	}
}

func TestUserResource_Delete_APIError(t *testing.T) {
	r := userResource(&services.MockUserService{
		DeleteFunc: func(ctx context.Context, id int64, deleteGroup bool) error {
			return errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, "71"),
		"delete_group": tftypes.NewValue(tftypes.Bool, true),
	})

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API failure")
	}
}

func TestUserResource_Delete_InvalidID(t *testing.T) {
	r := userResource(&services.MockUserService{})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "not-a-number"),
	})

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unparsable ID")
	}
}
