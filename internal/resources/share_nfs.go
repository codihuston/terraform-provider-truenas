package resources

import (
	"context"
	"fmt"
	"strconv"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &ShareNFSResource{}
	_ resource.ResourceWithConfigure   = &ShareNFSResource{}
	_ resource.ResourceWithImportState = &ShareNFSResource{}
)

// ShareNFSResourceModel describes the resource data model.
type ShareNFSResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Path            types.String `tfsdk:"path"`
	Aliases         types.List   `tfsdk:"aliases"`
	Comment         types.String `tfsdk:"comment"`
	Networks        types.List   `tfsdk:"networks"`
	Hosts           types.List   `tfsdk:"hosts"`
	ReadOnly        types.Bool   `tfsdk:"ro"`
	MaprootUser     types.String `tfsdk:"maproot_user"`
	MaprootGroup    types.String `tfsdk:"maproot_group"`
	MapallUser      types.String `tfsdk:"mapall_user"`
	MapallGroup     types.String `tfsdk:"mapall_group"`
	Security        types.List   `tfsdk:"security"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	ExposeSnapshots types.Bool   `tfsdk:"expose_snapshots"`
	Locked          types.Bool   `tfsdk:"locked"`
}

// ShareNFSResource defines the resource implementation.
type ShareNFSResource struct {
	BaseResource
}

// NewShareNFSResource creates a new ShareNFSResource.
func NewShareNFSResource() resource.Resource {
	return &ShareNFSResource{}
}

func (r *ShareNFSResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_share_nfs"
}

// emptyStringList is the default for the optional string list attributes,
// matching the API's own `[]` defaults.
func emptyStringList() types.List {
	return types.ListValueMust(types.StringType, []attr.Value{})
}

func (r *ShareNFSResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an NFS share (export). Creating a share does not start the NFS service; " +
			"enable the service separately.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NFS share ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Description: "Local path to be exported.",
				Required:    true,
			},
			"aliases": schema.ListAttribute{
				Description: "Share aliases. Read-only: TrueNAS accepts this field but discards " +
					"any value, so it is always empty.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"comment": schema.StringAttribute{
				Description: "User comment associated with the share.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"networks": schema.ListAttribute{
				Description: "Authorized networks allowed to access the share, in \"network/mask\" CIDR " +
					"notation. If empty, all networks are allowed.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(emptyStringList()),
			},
			"hosts": schema.ListAttribute{
				Description: "IP addresses or hostnames allowed to access the share. " +
					"If empty, all hosts are allowed.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(emptyStringList()),
			},
			"ro": schema.BoolAttribute{
				Description: "Export the share as read only.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"maproot_user": schema.StringAttribute{
				Description: "Map the root user of clients to this user. Conflicts with the mapall attributes.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("mapall_user"),
						path.MatchRoot("mapall_group"),
					),
				},
			},
			"maproot_group": schema.StringAttribute{
				Description: "Map the root group of clients to this group. Requires `maproot_user`.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("maproot_user")),
				},
			},
			"mapall_user": schema.StringAttribute{
				Description: "Map all client users to this user. Conflicts with the maproot attributes.",
				Optional:    true,
			},
			"mapall_group": schema.StringAttribute{
				Description: "Map all client groups to this group. Requires `mapall_user`.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("mapall_user")),
				},
			},
			"security": schema.ListAttribute{
				Description: "Security schemes offered by the export, in order of preference. " +
					"Valid values: SYS, KRB5, KRB5I, KRB5P.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(emptyStringList()),
				Validators: []validator.List{
					listvalidator.ValueStringsAre(
						stringvalidator.OneOf("SYS", "KRB5", "KRB5I", "KRB5P"),
					),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "Enable or disable the share.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"expose_snapshots": schema.BoolAttribute{
				Description: "Expose the ZFS snapshot directory over NFS. TrueNAS Enterprise only; " +
					"the export path must be the root of a ZFS dataset.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"locked": schema.BoolAttribute{
				Description: "Whether the share is located on a locked dataset.",
				Computed:    true,
			},
		},
	}
}

// buildShareNFSOpts builds typed options from the resource model.
func buildShareNFSOpts(ctx context.Context, data *ShareNFSResourceModel) (services.CreateNFSShareOpts, diag.Diagnostics) {
	var diags diag.Diagnostics

	opts := services.CreateNFSShareOpts{
		Path:            data.Path.ValueString(),
		Comment:         data.Comment.ValueString(),
		ReadOnly:        data.ReadOnly.ValueBool(),
		MaprootUser:     optionalString(data.MaprootUser),
		MaprootGroup:    optionalString(data.MaprootGroup),
		MapallUser:      optionalString(data.MapallUser),
		MapallGroup:     optionalString(data.MapallGroup),
		Enabled:         data.Enabled.ValueBool(),
		ExposeSnapshots: data.ExposeSnapshots.ValueBool(),
	}

	opts.Networks = stringsFromList(ctx, data.Networks, &diags)
	opts.Hosts = stringsFromList(ctx, data.Hosts, &diags)
	opts.Security = stringsFromList(ctx, data.Security, &diags)

	return opts, diags
}

func (r *ShareNFSResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ShareNFSResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, diags := buildShareNFSOpts(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	share, err := r.services.SharingNFS.Create(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create NFS Share",
			fmt.Sprintf("Unable to create NFS share: %s", err.Error()),
		)
		return
	}

	if share == nil {
		resp.Diagnostics.AddError(
			"NFS Share Not Found",
			"NFS share was created but could not be found.",
		)
		return
	}

	mapShareNFSToModel(share, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ShareNFSResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ShareNFSResourceModel

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

	share, err := r.services.SharingNFS.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read NFS Share",
			fmt.Sprintf("Unable to query NFS share: %s", err.Error()),
		)
		return
	}

	if share == nil {
		// Share was deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	mapShareNFSToModel(share, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ShareNFSResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state ShareNFSResourceModel
	var plan ShareNFSResourceModel

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

	opts, diags := buildShareNFSOpts(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	share, err := r.services.SharingNFS.Update(ctx, id, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update NFS Share",
			fmt.Sprintf("Unable to update NFS share: %s", err.Error()),
		)
		return
	}

	if share == nil {
		resp.Diagnostics.AddError(
			"NFS Share Not Found",
			"NFS share was updated but could not be found.",
		)
		return
	}

	mapShareNFSToModel(share, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ShareNFSResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ShareNFSResourceModel

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

	if err := r.services.SharingNFS.Delete(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete NFS Share",
			fmt.Sprintf("Unable to delete NFS share: %s", err.Error()),
		)
		return
	}
}

// mapShareNFSToModel maps a typed NFSShare to the resource model.
func mapShareNFSToModel(share *services.NFSShare, data *ShareNFSResourceModel) {
	data.ID = types.StringValue(strconv.FormatInt(share.ID, 10))
	data.Path = types.StringValue(share.Path)
	data.Comment = types.StringValue(share.Comment)
	data.ReadOnly = types.BoolValue(share.ReadOnly)
	data.MaprootUser = stringPointerValue(share.MaprootUser)
	data.MaprootGroup = stringPointerValue(share.MaprootGroup)
	data.MapallUser = stringPointerValue(share.MapallUser)
	data.MapallGroup = stringPointerValue(share.MapallGroup)
	data.Enabled = types.BoolValue(share.Enabled)
	data.ExposeSnapshots = types.BoolValue(share.ExposeSnapshots)

	if share.Locked != nil {
		data.Locked = types.BoolValue(*share.Locked)
	} else {
		data.Locked = types.BoolNull()
	}

	data.Aliases = listFromStrings(share.Aliases)
	data.Networks = listFromStrings(share.Networks)
	data.Hosts = listFromStrings(share.Hosts)
	data.Security = listFromStrings(share.Security)
}
