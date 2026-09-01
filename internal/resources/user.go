package resources

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &UserResource{}
	_ resource.ResourceWithConfigure      = &UserResource{}
	_ resource.ResourceWithImportState    = &UserResource{}
	_ resource.ResourceWithValidateConfig = &UserResource{}
)

// UserResourceModel describes the resource data model.
type UserResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Username             types.String `tfsdk:"username"`
	FullName             types.String `tfsdk:"full_name"`
	UID                  types.Int64  `tfsdk:"uid"`
	Group                types.Int64  `tfsdk:"group"`
	GroupCreate          types.Bool   `tfsdk:"group_create"`
	Groups               types.Set    `tfsdk:"groups"`
	Home                 types.String `tfsdk:"home"`
	HomeCreate           types.Bool   `tfsdk:"home_create"`
	HomeMode             types.String `tfsdk:"home_mode"`
	HomePath             types.String `tfsdk:"home_path"`
	Shell                types.String `tfsdk:"shell"`
	Email                types.String `tfsdk:"email"`
	SSHPublicKey         types.String `tfsdk:"ssh_public_key"`
	SSHPasswordEnabled   types.Bool   `tfsdk:"ssh_password_enabled"`
	SMB                  types.Bool   `tfsdk:"smb"`
	Locked               types.Bool   `tfsdk:"locked"`
	PasswordDisabled     types.Bool   `tfsdk:"password_disabled"`
	Password             types.String `tfsdk:"password"`
	PasswordWOVersion    types.Int64  `tfsdk:"password_wo_version"`
	SudoCommands         types.List   `tfsdk:"sudo_commands"`
	SudoCommandsNoPasswd types.List   `tfsdk:"sudo_commands_nopasswd"`
	DeleteGroup          types.Bool   `tfsdk:"delete_group"`
	Builtin              types.Bool   `tfsdk:"builtin"`
}

// UserResource defines the resource implementation.
type UserResource struct {
	BaseResource
}

// NewUserResource creates a new UserResource.
func NewUserResource() resource.Resource {
	return &UserResource{}
}

func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a local user account. Accounts are created without password " +
			"authentication unless `password` is supplied, so the default account " +
			"authenticates by SSH key or not at all.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "User entry ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				Description: "Login name. Limited to alphanumerics, hyphens, underscores and periods, " +
					"and may not begin with a hyphen or a period.",
				Required: true,
			},
			"full_name": schema.StringAttribute{
				Description: "Descriptive name for the account, such as the person's full name or the " +
					"purpose of a service account.",
				Required: true,
			},
			"uid": schema.Int64Attribute{
				Description: "Unix user ID. Defaults to the next available UID. Changing this forces a new user.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"group": schema.Int64Attribute{
				Description: "Entry ID of the primary group, as exported by `truenas_group.<name>.id`. " +
					"This is the group entry ID, not the Unix GID. Required unless `group_create` is set.",
				Optional: true,
				Computed: true,
			},
			"group_create": schema.BoolAttribute{
				Description: "Create a new group named after the user and use it as the primary group. " +
					"Only honoured when the user is created; TrueNAS rejects it on update.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"groups": schema.SetAttribute{
				Description: "Entry IDs of additional groups the user belongs to. TrueNAS adds SMB " +
					"users to `builtin_users` itself, and that group alone is ignored when this " +
					"attribute is reconciled; every other membership change made outside Terraform " +
					"is reported as drift. Leave unset to let TrueNAS manage membership entirely, " +
					"or set to `[]` to remove all additional groups.",
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
			},
			"home": schema.StringAttribute{
				Description: "Home directory to assign to the user. When `home_create` is set this is " +
					"the parent directory and TrueNAS creates the home at `<home>/<username>`; " +
					"the resulting path is exported as `home_path`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("/var/empty"),
			},
			"home_create": schema.BoolAttribute{
				Description: "Create the home directory under `home`. Only sent when `home` changes, so " +
					"repeated applies do not move or nest the existing home.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"home_mode": schema.StringAttribute{
				Description: "Octal permissions applied to the home directory. TrueNAS does not report " +
					"this back, so changes made outside Terraform are not detected.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("700"),
			},
			"home_path": schema.StringAttribute{
				Description: "Home directory TrueNAS assigned to the user.",
				Computed:    true,
			},
			"shell": schema.StringAttribute{
				Description: "Login shell. Valid choices come from the `user.shell_choices` API method, " +
					"for example `/usr/bin/bash` or `/usr/sbin/nologin`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("/usr/bin/zsh"),
			},
			"email": schema.StringAttribute{
				Description: "Email address for the account. Accounts with the `FULL_ADMIN` role receive " +
					"alerts and notifications at this address.",
				Optional: true,
			},
			"ssh_public_key": schema.StringAttribute{
				Description: "SSH public keys authorised for this account. TrueNAS writes these to the " +
					"user's home directory, so `home` must point at a writable path.",
				Optional: true,
			},
			"ssh_password_enabled": schema.BoolAttribute{
				Description: "Allow SSH password authentication. Leave disabled and use `ssh_public_key` " +
					"unless password logins are specifically required.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"smb": schema.BoolAttribute{
				Description: "Allow the account to access SMB shares. TrueNAS requires a password for SMB " +
					"users and adds them to `builtin_users`, so this defaults to disabled.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"locked": schema.BoolAttribute{
				Description: "Lock the account so it cannot authenticate.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"password_disabled": schema.BoolAttribute{
				Description: "Disable password authentication. The account can still authenticate by " +
					"other means, such as an SSH key. Cannot be combined with `smb`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"password": schema.StringAttribute{
				Description: "Password for the account. This is a write-only attribute: it is never " +
					"written to state and never read back, so a password changed outside Terraform " +
					"produces no drift. Increment `password_wo_version` to send it again.",
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
			},
			"password_wo_version": schema.Int64Attribute{
				Description: "Change this value to re-send `password` on the next apply. Because `password` " +
					"is write-only, this is the only way to trigger a password change.",
				Optional: true,
			},
			"sudo_commands": schema.ListAttribute{
				Description: "Commands the user may run with elevated privileges, prompting for a password.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Default:     listdefault.StaticValue(emptyStringList()),
			},
			"sudo_commands_nopasswd": schema.ListAttribute{
				Description: "Commands the user may run with elevated privileges without a password prompt.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Default:     listdefault.StaticValue(emptyStringList()),
			},
			"delete_group": schema.BoolAttribute{
				Description: "Delete the primary group along with the user, provided no other user " +
					"depends on it. Set this to `false` when the primary group is managed by a " +
					"separate `truenas_group` resource.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"builtin": schema.BoolAttribute{
				Description: "Whether the account is built in to TrueNAS.",
				Computed:    true,
			},
		},
	}
}

// ValidateConfig enforces the API rule that a primary group is either supplied
// or created, rejects the SMB and password_disabled combination up front so the
// error surfaces at plan time rather than mid-apply, and requires password and
// password_wo_version to be configured together so a password change cannot be
// silently dropped.
func (r *UserResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// An unknown group counts as set: it resolves to another resource's ID.
	groupSet := !data.Group.IsNull()
	groupCreate := data.GroupCreate.ValueBool()

	switch {
	case !groupSet && !groupCreate && !data.GroupCreate.IsUnknown():
		resp.Diagnostics.AddAttributeError(
			path.Root("group"),
			"Missing Primary Group",
			"Set group to an existing group entry ID or set group_create to true.",
		)
	case groupSet && groupCreate:
		resp.Diagnostics.AddAttributeError(
			path.Root("group"),
			"Conflicting Primary Group",
			"group and group_create are mutually exclusive; TrueNAS ignores group when creating a new primary group.",
		)
	}

	// An unknown value counts as set: it resolves to a variable or another
	// resource's attribute.
	passwordSet := !data.Password.IsNull()
	versionSet := !data.PasswordWOVersion.IsNull()

	switch {
	case passwordSet && !versionSet:
		resp.Diagnostics.AddAttributeError(
			path.Root("password_wo_version"),
			"Missing Password Version",
			"password is write-only, so password_wo_version is the only way to re-send it. Set password_wo_version alongside password.",
		)
	case !passwordSet && versionSet:
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Missing Password",
			"password_wo_version has no effect without password. Set password alongside password_wo_version.",
		)
	}

	if data.SMB.ValueBool() && data.PasswordDisabled.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("password_disabled"),
			"Password Authentication Required for SMB",
			"TrueNAS does not allow password authentication to be disabled for SMB users. Disable smb or leave password_disabled unset.",
		)
	}
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserResourceModel
	var config UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := services.CreateUserOpts{
		Username:             data.Username.ValueString(),
		FullName:             data.FullName.ValueString(),
		GroupCreate:          data.GroupCreate.ValueBool(),
		Groups:               int64SetValues(ctx, data.Groups, &resp.Diagnostics),
		Home:                 data.Home.ValueString(),
		HomeCreate:           data.HomeCreate.ValueBool(),
		HomeMode:             data.HomeMode.ValueString(),
		Shell:                data.Shell.ValueString(),
		Email:                optionalString(data.Email),
		SMB:                  data.SMB.ValueBool(),
		Locked:               data.Locked.ValueBool(),
		PasswordDisabled:     data.PasswordDisabled.ValueBool(),
		SSHPasswordEnabled:   data.SSHPasswordEnabled.ValueBool(),
		SSHPublicKey:         optionalString(data.SSHPublicKey),
		SudoCommands:         stringListValues(ctx, data.SudoCommands, &resp.Diagnostics),
		SudoCommandsNoPasswd: stringListValues(ctx, data.SudoCommandsNoPasswd, &resp.Diagnostics),
		Password:             optionalString(config.Password),
	}
	if !data.UID.IsNull() && !data.UID.IsUnknown() {
		uid := data.UID.ValueInt64()
		opts.UID = &uid
	}
	if !data.Group.IsNull() && !data.Group.IsUnknown() {
		group := data.Group.ValueInt64()
		opts.Group = &group
	}
	if resp.Diagnostics.HasError() {
		return
	}

	builtinUsers, diags := r.builtinUsersID(ctx, data.Groups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.services.User.Create(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create User",
			fmt.Sprintf("Unable to create user: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(mapUserToModel(ctx, user, &data, builtinUsers)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserResourceModel

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

	builtinUsers, diags := r.builtinUsersID(ctx, data.Groups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.services.User.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read User",
			fmt.Sprintf("Unable to query user: %s", err.Error()),
		)
		return
	}

	if user == nil {
		// User was deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(mapUserToModel(ctx, user, &data, builtinUsers)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state UserResourceModel
	var plan UserResourceModel
	var config UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
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

	// home is the parent directory when home_create is set, and TrueNAS rejects
	// that parent on update. Only a changed home is sent as configured; an
	// unchanged one is sent as the path the server actually assigned.
	home, homeCreate := state.HomePath.ValueString(), false
	if !plan.Home.Equal(state.Home) {
		home, homeCreate = plan.Home.ValueString(), plan.HomeCreate.ValueBool()
	}

	opts := services.UpdateUserOpts{
		Username:             plan.Username.ValueString(),
		FullName:             plan.FullName.ValueString(),
		Groups:               int64SetValues(ctx, plan.Groups, &resp.Diagnostics),
		Home:                 home,
		HomeCreate:           homeCreate,
		HomeMode:             plan.HomeMode.ValueString(),
		Shell:                plan.Shell.ValueString(),
		Email:                optionalString(plan.Email),
		SMB:                  plan.SMB.ValueBool(),
		Locked:               plan.Locked.ValueBool(),
		PasswordDisabled:     plan.PasswordDisabled.ValueBool(),
		SSHPasswordEnabled:   plan.SSHPasswordEnabled.ValueBool(),
		SSHPublicKey:         optionalString(plan.SSHPublicKey),
		SudoCommands:         stringListValues(ctx, plan.SudoCommands, &resp.Diagnostics),
		SudoCommandsNoPasswd: stringListValues(ctx, plan.SudoCommandsNoPasswd, &resp.Diagnostics),
	}
	if !plan.Group.IsNull() && !plan.Group.IsUnknown() {
		group := plan.Group.ValueInt64()
		opts.Group = &group
	}
	// password is write-only, so there is nothing in state to compare it
	// against. password_wo_version is the caller's signal to send it again.
	if !plan.PasswordWOVersion.Equal(state.PasswordWOVersion) {
		opts.Password = optionalString(config.Password)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	builtinUsers, diags := r.builtinUsersID(ctx, plan.Groups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.services.User.Update(ctx, id, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update User",
			fmt.Sprintf("Unable to update user: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(mapUserToModel(ctx, user, &plan, builtinUsers)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserResourceModel

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

	if err := r.services.User.Delete(ctx, id, data.DeleteGroup.ValueBool()); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete User",
			fmt.Sprintf("Unable to delete user: %s", err.Error()),
		)
	}
}

// mapUserToModel maps a typed User to the resource model.
//
// home, home_create and home_mode are inputs TrueNAS does not report back and
// are left as configured. home_path carries the directory the server actually
// assigned, which differs from home whenever home_create was used.
func mapUserToModel(ctx context.Context, user *services.User, data *UserResourceModel, builtinUsers *int64) diag.Diagnostics {
	var diags diag.Diagnostics

	// An import has no configured home to preserve, so seed it from the
	// assigned path. That matches the configuration exactly unless the home
	// directory was created by TrueNAS rather than supplied verbatim.
	if data.Home.IsNull() {
		data.Home = types.StringValue(user.Home)
	}

	data.ID = types.StringValue(strconv.FormatInt(user.ID, 10))
	data.Username = types.StringValue(user.Username)
	data.FullName = types.StringValue(user.FullName)
	data.UID = types.Int64Value(user.UID)
	data.Group = types.Int64Value(user.Group)
	data.HomePath = types.StringValue(user.Home)
	data.Shell = types.StringValue(user.Shell)
	data.SMB = types.BoolValue(user.SMB)
	data.Locked = types.BoolValue(user.Locked)
	data.PasswordDisabled = types.BoolValue(user.PasswordDisabled)
	data.SSHPasswordEnabled = types.BoolValue(user.SSHPasswordEnabled)
	data.Builtin = types.BoolValue(user.Builtin)
	data.Email = normalizedStringValue(data.Email, user.Email)
	data.SSHPublicKey = normalizedStringValue(data.SSHPublicKey, user.SSHPublicKey)

	data.Groups = reconcileGroups(ctx, data.Groups, user.Groups, builtinUsers, &diags)

	sudoCommands, d := types.ListValueFrom(ctx, types.StringType, user.SudoCommands)
	diags.Append(d...)
	data.SudoCommands = sudoCommands

	sudoCommandsNoPasswd, d := types.ListValueFrom(ctx, types.StringType, user.SudoCommandsNoPasswd)
	diags.Append(d...)
	data.SudoCommandsNoPasswd = sudoCommandsNoPasswd

	return diags
}

// optionalStringValue converts an API string back to an optional attribute,
// mapping the empty string to null so an unset attribute stays unset.
func optionalStringValue(s string) types.String {
	if s == "" {
		return types.StringNull()
	}

	return types.StringValue(s)
}

// builtinUsersID resolves the builtin_users group, returning nil when the
// server does not have one so reconciliation falls back to a plain comparison.
// The lookup is skipped unless there is a membership to reconcile against, so
// an unset groups keeps user operations independent of group.query.
func (r *UserResource) builtinUsersID(ctx context.Context, groups types.Set) (*int64, diag.Diagnostics) {
	var diags diag.Diagnostics

	if groups.IsNull() || groups.IsUnknown() {
		return nil, diags
	}

	id, found, err := r.services.Group.BuiltinUsersID(ctx)
	if err != nil {
		diags.AddError(
			"Unable to Read Groups",
			fmt.Sprintf("Unable to query the builtin_users group: %s", err.Error()),
		)
		return nil, diags
	}
	if !found {
		return nil, diags
	}

	return &id, diags
}

// reconcileGroups keeps a configured membership when the server's set differs
// from it only by builtin_users. Computed relaxes Terraform's final == planned
// check only while the planned value is unknown, so an explicitly configured
// groups is known, and TrueNAS adding an SMB user to builtin_users on its own
// would otherwise abort the apply with "Provider produced inconsistent result
// after apply". Only that group is excused: every other difference is taken
// from the server so an out-of-band membership change surfaces as drift.
func reconcileGroups(ctx context.Context, configured types.Set, groups []int64, builtinUsers *int64, diags *diag.Diagnostics) types.Set {
	if !configured.IsNull() && !configured.IsUnknown() &&
		sameGroups(int64SetValues(ctx, configured, diags), groups, builtinUsers) {
		return configured
	}

	value, d := types.SetValueFrom(ctx, types.Int64Type, groups)
	diags.Append(d...)

	return value
}

// sameGroups reports whether the stored and reported memberships match. The
// builtin_users exemption is one-directional: it excuses TrueNAS adding the
// group, so it applies only to a group the server reports and the stored value
// does not have. Losing a membership the stored value records is a real change
// and surfaces like any other.
func sameGroups(stored, reported []int64, builtinUsers *int64) bool {
	set := func(ids []int64) map[int64]struct{} {
		out := make(map[int64]struct{}, len(ids))
		for _, id := range ids {
			out[id] = struct{}{}
		}
		return out
	}

	left, right := set(stored), set(reported)
	if builtinUsers != nil {
		if _, ok := left[*builtinUsers]; !ok {
			delete(right, *builtinUsers)
		}
	}

	if len(left) != len(right) {
		return false
	}
	for id := range left {
		if _, ok := right[id]; !ok {
			return false
		}
	}

	return true
}

// normalizedStringValue keeps the configured value when the server returns an
// equivalent one. TrueNAS trims sshpubkey whitespace, and these attributes are
// optional rather than computed, so the post-apply state has to equal the
// configuration exactly or Terraform aborts with "Provider produced
// inconsistent result after apply" — the shipped example feeds
// `file("deploy.pub")`, which ends in a newline. A materially different value
// still replaces the stored one, so out-of-band drift surfaces on read.
func normalizedStringValue(configured types.String, s string) types.String {
	if !configured.IsNull() && !configured.IsUnknown() &&
		strings.TrimSpace(configured.ValueString()) == strings.TrimSpace(s) {
		return configured
	}

	return optionalStringValue(s)
}

// int64SetValues converts a set attribute to a Go slice, treating null and
// unknown as empty.
func int64SetValues(ctx context.Context, set types.Set, diags *diag.Diagnostics) []int64 {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}

	var values []int64
	diags.Append(set.ElementsAs(ctx, &values, false)...)
	return values
}
