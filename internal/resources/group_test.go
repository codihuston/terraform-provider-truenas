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

func testGroup() *services.Group {
	return &services.Group{
		ID:                   110,
		GID:                  3000,
		Name:                 "developers",
		SMB:                  true,
		SudoCommands:         []string{"/usr/bin/systemctl"},
		SudoCommandsNoPasswd: []string{},
	}
}

// groupResource builds a group resource backed by the supplied mock service.
func groupResource(mock *services.MockGroupService) *GroupResource {
	return &GroupResource{
		BaseResource: BaseResource{services: &services.TrueNASServices{Group: mock}},
	}
}

func TestNewGroupResource(t *testing.T) {
	r := NewGroupResource()

	g, ok := r.(*GroupResource)
	if !ok {
		t.Fatalf("expected *GroupResource, got %T", r)
	}

	_ = resource.ResourceWithConfigure(g)
	_ = resource.ResourceWithImportState(g)
}

func TestGroupResource_Metadata(t *testing.T) {
	resp := &resource.MetadataResponse{}
	NewGroupResource().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "truenas"}, resp)

	if resp.TypeName != "truenas_group" {
		t.Errorf("expected TypeName 'truenas_group', got %q", resp.TypeName)
	}
}

func TestGroupResource_Schema(t *testing.T) {
	s := resourceSchema(t, NewGroupResource())

	if s.Description == "" {
		t.Error("expected non-empty schema description")
	}

	for _, name := range []string{"id", "name", "gid", "smb", "sudo_commands", "sudo_commands_nopasswd", "builtin"} {
		if s.Attributes[name] == nil {
			t.Errorf("expected %q attribute", name)
		}
	}
}

func TestGroupResource_Create_Success(t *testing.T) {
	var captured services.CreateGroupOpts

	r := groupResource(&services.MockGroupService{
		CreateFunc: func(ctx context.Context, opts services.CreateGroupOpts) (*services.Group, error) {
			captured = opts
			return testGroup(), nil
		},
	})

	s := resourceSchema(t, r)
	plan := objectValue(t, s, map[string]tftypes.Value{
		"name":                   tftypes.NewValue(tftypes.String, "developers"),
		"gid":                    tftypes.NewValue(tftypes.Number, 3000),
		"smb":                    tftypes.NewValue(tftypes.Bool, true),
		"sudo_commands":          stringList("/usr/bin/systemctl"),
		"sudo_commands_nopasswd": stringList(),
	})

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: plan}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	if captured.Name != "developers" {
		t.Errorf("expected name 'developers', got %q", captured.Name)
	}
	if captured.GID == nil || *captured.GID != 3000 {
		t.Errorf("expected gid 3000, got %v", captured.GID)
	}
	if !captured.SMB {
		t.Error("expected smb true")
	}
	if len(captured.SudoCommands) != 1 || captured.SudoCommands[0] != "/usr/bin/systemctl" {
		t.Errorf("expected one sudo command, got %v", captured.SudoCommands)
	}

	var state GroupResourceModel
	resp.State.Get(context.Background(), &state)
	if state.ID.ValueString() != "110" {
		t.Errorf("expected ID '110', got %q", state.ID.ValueString())
	}
	if state.GID.ValueInt64() != 3000 {
		t.Errorf("expected gid 3000, got %d", state.GID.ValueInt64())
	}
	if state.Builtin.ValueBool() {
		t.Error("expected builtin false")
	}
}

func TestGroupResource_Create_OmitsUnknownGID(t *testing.T) {
	var captured services.CreateGroupOpts

	r := groupResource(&services.MockGroupService{
		CreateFunc: func(ctx context.Context, opts services.CreateGroupOpts) (*services.Group, error) {
			captured = opts
			return testGroup(), nil
		},
	})

	s := resourceSchema(t, r)
	plan := objectValue(t, s, map[string]tftypes.Value{
		"name":                   tftypes.NewValue(tftypes.String, "developers"),
		"gid":                    tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
		"smb":                    tftypes.NewValue(tftypes.Bool, true),
		"sudo_commands":          stringList(),
		"sudo_commands_nopasswd": stringList(),
	})

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: plan}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if captured.GID != nil {
		t.Errorf("expected gid to be omitted, got %d", *captured.GID)
	}
}

func TestGroupResource_Create_APIError(t *testing.T) {
	r := groupResource(&services.MockGroupService{
		CreateFunc: func(ctx context.Context, opts services.CreateGroupOpts) (*services.Group, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	plan := objectValue(t, s, map[string]tftypes.Value{
		"name":                   tftypes.NewValue(tftypes.String, "developers"),
		"smb":                    tftypes.NewValue(tftypes.Bool, true),
		"sudo_commands":          stringList(),
		"sudo_commands_nopasswd": stringList(),
	})

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: plan}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API failure")
	}
}

func TestGroupResource_Read_Success(t *testing.T) {
	r := groupResource(&services.MockGroupService{
		GetFunc: func(ctx context.Context, id int64) (*services.Group, error) {
			if id != 110 {
				t.Errorf("expected id 110, got %d", id)
			}
			return testGroup(), nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "110"),
	})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}

	var result GroupResourceModel
	resp.State.Get(context.Background(), &result)
	if result.Name.ValueString() != "developers" {
		t.Errorf("expected name 'developers', got %q", result.Name.ValueString())
	}
}

func TestGroupResource_Read_NotFound(t *testing.T) {
	r := groupResource(&services.MockGroupService{
		GetFunc: func(ctx context.Context, id int64) (*services.Group, error) {
			return nil, nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "110"),
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

func TestGroupResource_Read_APIError(t *testing.T) {
	r := groupResource(&services.MockGroupService{
		GetFunc: func(ctx context.Context, id int64) (*services.Group, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "110"),
	})

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Read(context.Background(), resource.ReadRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API failure")
	}
}

func TestGroupResource_Read_InvalidID(t *testing.T) {
	r := groupResource(&services.MockGroupService{})

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

func TestGroupResource_Update_Success(t *testing.T) {
	var captured services.UpdateGroupOpts
	var capturedID int64

	r := groupResource(&services.MockGroupService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateGroupOpts) (*services.Group, error) {
			capturedID = id
			captured = opts
			group := testGroup()
			group.Name = "engineers"
			return group, nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, "110"),
		"name": tftypes.NewValue(tftypes.String, "developers"),
	})
	plan := objectValue(t, s, map[string]tftypes.Value{
		"id":                     tftypes.NewValue(tftypes.String, "110"),
		"name":                   tftypes.NewValue(tftypes.String, "engineers"),
		"smb":                    tftypes.NewValue(tftypes.Bool, true),
		"sudo_commands":          stringList("/usr/bin/systemctl"),
		"sudo_commands_nopasswd": stringList(),
	})

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: s, Raw: state},
		Plan:  tfsdk.Plan{Schema: s, Raw: plan},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 110 {
		t.Errorf("expected id 110, got %d", capturedID)
	}
	if captured.Name != "engineers" {
		t.Errorf("expected name 'engineers', got %q", captured.Name)
	}

	var result GroupResourceModel
	resp.State.Get(context.Background(), &result)
	if result.Name.ValueString() != "engineers" {
		t.Errorf("expected name 'engineers', got %q", result.Name.ValueString())
	}
}

func TestGroupResource_Update_APIError(t *testing.T) {
	r := groupResource(&services.MockGroupService{
		UpdateFunc: func(ctx context.Context, id int64, opts services.UpdateGroupOpts) (*services.Group, error) {
			return nil, errors.New("connection refused")
		},
	})

	s := resourceSchema(t, r)
	value := objectValue(t, s, map[string]tftypes.Value{
		"id":                     tftypes.NewValue(tftypes.String, "110"),
		"name":                   tftypes.NewValue(tftypes.String, "engineers"),
		"smb":                    tftypes.NewValue(tftypes.Bool, true),
		"sudo_commands":          stringList(),
		"sudo_commands_nopasswd": stringList(),
	})

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: value}}
	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: s, Raw: value},
		Plan:  tfsdk.Plan{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API failure")
	}
}

func TestGroupResource_Update_InvalidID(t *testing.T) {
	r := groupResource(&services.MockGroupService{})

	s := resourceSchema(t, r)
	value := objectValue(t, s, map[string]tftypes.Value{
		"id":                     tftypes.NewValue(tftypes.String, "not-a-number"),
		"name":                   tftypes.NewValue(tftypes.String, "engineers"),
		"smb":                    tftypes.NewValue(tftypes.Bool, true),
		"sudo_commands":          stringList(),
		"sudo_commands_nopasswd": stringList(),
	})

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: value}}
	r.Update(context.Background(), resource.UpdateRequest{
		State: tfsdk.State{Schema: s, Raw: value},
		Plan:  tfsdk.Plan{Schema: s, Raw: value},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for unparsable ID")
	}
}

func TestGroupResource_Delete_Success(t *testing.T) {
	var capturedID int64

	r := groupResource(&services.MockGroupService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			capturedID = id
			return nil
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "110"),
	})

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected errors: %v", resp.Diagnostics)
	}
	if capturedID != 110 {
		t.Errorf("expected id 110, got %d", capturedID)
	}
}

func TestGroupResource_Delete_APIError(t *testing.T) {
	r := groupResource(&services.MockGroupService{
		DeleteFunc: func(ctx context.Context, id int64) error {
			return errors.New("group is a primary group")
		},
	})

	s := resourceSchema(t, r)
	state := objectValue(t, s, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "110"),
	})

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: s, Raw: state}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for API failure")
	}
}

func TestGroupResource_Delete_InvalidID(t *testing.T) {
	r := groupResource(&services.MockGroupService{})

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
