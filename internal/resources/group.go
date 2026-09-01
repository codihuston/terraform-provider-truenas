package resources

import (
	"context"
	"fmt"
	"strconv"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &GroupResource{}
	_ resource.ResourceWithConfigure   = &GroupResource{}
	_ resource.ResourceWithImportState = &GroupResource{}
)

// GroupResourceModel describes the resource data model.
type GroupResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	GID                  types.Int64  `tfsdk:"gid"`
	SMB                  types.Bool   `tfsdk:"smb"`
	SudoCommands         types.List   `tfsdk:"sudo_commands"`
	SudoCommandsNoPasswd types.List   `tfsdk:"sudo_commands_nopasswd"`
	Builtin              types.Bool   `tfsdk:"builtin"`
}

// GroupResource defines the resource implementation.
type GroupResource struct {
	BaseResource
}

// NewGroupResource creates a new GroupResource.
func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

func (r *GroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *GroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a local group. Use the group's `id` for the `group` and `groups` " +
			"attributes of `truenas_user`, which take group entry IDs rather than Unix GIDs.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Group entry ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Group name.",
				Required:    true,
			},
			"gid": schema.Int64Attribute{
				Description: "Unix group ID. Defaults to the next available GID. Changing this forces a new group.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"smb": schema.BoolAttribute{
				Description: "Allow the group to be used in SMB share ACL entries.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"sudo_commands": schema.ListAttribute{
				Description: "Commands group members may run with elevated privileges, prompting for a password.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Default:     listdefault.StaticValue(emptyStringList()),
			},
			"sudo_commands_nopasswd": schema.ListAttribute{
				Description: "Commands group members may run with elevated privileges without a password prompt.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Default:     listdefault.StaticValue(emptyStringList()),
			},
			"builtin": schema.BoolAttribute{
				Description: "Whether the group is built in to TrueNAS.",
				Computed:    true,
			},
		},
	}
}

func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := services.CreateGroupOpts{
		Name:                 data.Name.ValueString(),
		SMB:                  data.SMB.ValueBool(),
		SudoCommands:         stringListValues(ctx, data.SudoCommands, &resp.Diagnostics),
		SudoCommandsNoPasswd: stringListValues(ctx, data.SudoCommandsNoPasswd, &resp.Diagnostics),
	}
	if !data.GID.IsNull() && !data.GID.IsUnknown() {
		gid := data.GID.ValueInt64()
		opts.GID = &gid
	}
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.services.Group.Create(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Group",
			fmt.Sprintf("Unable to create group: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(mapGroupToModel(ctx, group, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid ID",
			fmt.Sprintf("Unable to parse ID %q: %s", data.ID.ValueString(), err.Error()),
		)
		return
	}

	group, err := r.services.Group.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Group",
			fmt.Sprintf("Unable to query group: %s", err.Error()),
		)
		return
	}

	if group == nil {
		// Group was deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(mapGroupToModel(ctx, group, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state GroupResourceModel
	var plan GroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid ID",
			fmt.Sprintf("Unable to parse ID %q: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	opts := services.UpdateGroupOpts{
		Name:                 plan.Name.ValueString(),
		SMB:                  plan.SMB.ValueBool(),
		SudoCommands:         stringListValues(ctx, plan.SudoCommands, &resp.Diagnostics),
		SudoCommandsNoPasswd: stringListValues(ctx, plan.SudoCommandsNoPasswd, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.services.Group.Update(ctx, id, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update Group",
			fmt.Sprintf("Unable to update group: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(mapGroupToModel(ctx, group, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid ID",
			fmt.Sprintf("Unable to parse ID %q: %s", data.ID.ValueString(), err.Error()),
		)
		return
	}

	if err := r.services.Group.Delete(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete Group",
			fmt.Sprintf("Unable to delete group: %s", err.Error()),
		)
	}
}

// mapGroupToModel maps a typed Group to the resource model.
func mapGroupToModel(ctx context.Context, group *services.Group, data *GroupResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ID = types.StringValue(strconv.FormatInt(group.ID, 10))
	data.Name = types.StringValue(group.Name)
	data.GID = types.Int64Value(group.GID)
	data.SMB = types.BoolValue(group.SMB)
	data.Builtin = types.BoolValue(group.Builtin)

	sudoCommands, d := types.ListValueFrom(ctx, types.StringType, group.SudoCommands)
	diags.Append(d...)
	data.SudoCommands = sudoCommands

	sudoCommandsNoPasswd, d := types.ListValueFrom(ctx, types.StringType, group.SudoCommandsNoPasswd)
	diags.Append(d...)
	data.SudoCommandsNoPasswd = sudoCommandsNoPasswd

	return diags
}

// stringListValues converts a list attribute to a Go slice, treating null and
// unknown as empty.
func stringListValues(ctx context.Context, list types.List, diags *diag.Diagnostics) []string {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}

	var values []string
	diags.Append(list.ElementsAs(ctx, &values, false)...)
	return values
}
