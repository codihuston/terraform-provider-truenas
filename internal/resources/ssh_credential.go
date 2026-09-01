package resources

import (
	"context"
	"fmt"
	"strconv"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &SSHCredentialResource{}
	_ resource.ResourceWithConfigure   = &SSHCredentialResource{}
	_ resource.ResourceWithImportState = &SSHCredentialResource{}
)

// SSHCredentialResourceModel describes the resource data model.
type SSHCredentialResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Host           types.String `tfsdk:"host"`
	Port           types.Int64  `tfsdk:"port"`
	Username       types.String `tfsdk:"username"`
	PrivateKeyID   types.String `tfsdk:"private_key_id"`
	RemoteHostKey  types.String `tfsdk:"remote_host_key"`
	ConnectTimeout types.Int64  `tfsdk:"connect_timeout"`
}

// SSHCredentialResource defines the resource implementation.
type SSHCredentialResource struct {
	BaseResource
}

// NewSSHCredentialResource creates a new SSHCredentialResource.
func NewSSHCredentialResource() resource.Resource {
	return &SSHCredentialResource{}
}

func (r *SSHCredentialResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_credential"
}

func (r *SSHCredentialResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an SSH connection in the TrueNAS keychain: a remote host, the account " +
			"to log in as, and the `truenas_ssh_keypair` to authenticate with. Replication " +
			"tasks and other features reference a connection by its ID rather than carrying " +
			"the credentials themselves.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Keychain credential ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name that distinguishes this connection from others in the keychain.",
				Required:    true,
			},
			"host": schema.StringAttribute{
				Description: "Hostname or IP address of the remote SSH server.",
				Required:    true,
			},
			"port": schema.Int64Attribute{
				Description: "Port the remote SSH server listens on.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(22),
				Validators: []validator.Int64{
					int64validator.Between(1, 65535),
				},
			},
			"username": schema.StringAttribute{
				Description: "Account to log in to the remote host as.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("root"),
			},
			"private_key_id": schema.StringAttribute{
				Description: "ID of the `truenas_ssh_keypair` this connection authenticates with. " +
					"TrueNAS does not check that the ID names an existing key pair, so a wrong " +
					"ID surfaces only when something tries to use the connection.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"remote_host_key": schema.StringAttribute{
				Description: "Public host keys the remote host is trusted to present, one per line, " +
					"in `known_hosts` format without the leading host field. Leave it unset to " +
					"have TrueNAS scan the host once, when the connection is created, and trust " +
					"whatever answers — an unchanged host is not re-verified afterwards, so a host " +
					"key that changes later is neither detected nor reported as drift. Changing " +
					"`host` or `port` while this attribute is unset plans it as known after apply " +
					"and scans afresh, which is a new trust-on-first-use event with the same " +
					"implications as the first: the new host is trusted on whatever answers at " +
					"that moment, and nothing verifies it is the intended machine.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					remoteHostKeyPlanModifier(),
				},
			},
			"connect_timeout": schema.Int64Attribute{
				Description: "Seconds to wait for the remote host to accept a connection.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(10),
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
		},
	}
}

func (r *SSHCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SSHCredentialResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, ok := r.buildSSHCredentialOpts(ctx, &data, data.RemoteHostKey.IsUnknown(), &resp.Diagnostics)
	if !ok {
		return
	}

	credential, err := r.services.KeychainCredential.CreateSSHCredential(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create SSH Credential",
			fmt.Sprintf("Unable to create SSH credential: %s", err.Error()),
		)
		return
	}

	mapSSHCredentialToModel(credential, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSHCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SSHCredentialResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, ok := parseKeychainCredentialID(data.ID, &resp.Diagnostics)
	if !ok {
		return
	}

	credential, err := r.services.KeychainCredential.GetSSHCredential(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read SSH Credential",
			fmt.Sprintf("Unable to query SSH credential: %s", err.Error()),
		)
		return
	}

	if credential == nil {
		// Credential was deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	mapSSHCredentialToModel(credential, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSHCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state SSHCredentialResourceModel
	var plan SSHCredentialResourceModel
	var config SSHCredentialResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, ok := parseKeychainCredentialID(state.ID, &resp.Diagnostics)
	if !ok {
		return
	}

	// The stored host key must always belong to the recorded host. When the
	// provider owns the key — the configuration leaves it out — moving the
	// connection to another host or port means scanning that host afresh
	// rather than resubmitting the key scanned for the previous one.
	unmanagedHostKey := config.RemoteHostKey.IsNull()
	movedHost := !plan.Host.Equal(state.Host) || !plan.Port.Equal(state.Port)
	scan := plan.RemoteHostKey.IsUnknown() || (unmanagedHostKey && movedHost)

	opts, ok := r.buildSSHCredentialOpts(ctx, &plan, scan, &resp.Diagnostics)
	if !ok {
		return
	}

	credential, err := r.services.KeychainCredential.UpdateSSHCredential(ctx, id, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update SSH Credential",
			fmt.Sprintf("Unable to update SSH credential: %s", err.Error()),
		)
		return
	}

	mapSSHCredentialToModel(credential, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SSHCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SSHCredentialResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, ok := parseKeychainCredentialID(data.ID, &resp.Diagnostics)
	if !ok {
		return
	}

	if err := r.services.KeychainCredential.Delete(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete SSH Credential",
			fmt.Sprintf("Unable to delete SSH credential: %s", err.Error()),
		)
	}
}

// buildSSHCredentialOpts builds typed options from the resource model,
// discovering the remote host key when the caller asks for a scan.
func (r *SSHCredentialResource) buildSSHCredentialOpts(ctx context.Context, data *SSHCredentialResourceModel, scan bool, diags *diag.Diagnostics) (services.CreateSSHCredentialOpts, bool) {
	var opts services.CreateSSHCredentialOpts

	privateKeyID, err := strconv.ParseInt(data.PrivateKeyID.ValueString(), 10, 64)
	if err != nil {
		diags.AddError(
			"Invalid Private Key ID",
			fmt.Sprintf("Unable to parse private_key_id %q as a keychain credential ID: %s", data.PrivateKeyID.ValueString(), err.Error()),
		)
		return opts, false
	}

	opts = services.CreateSSHCredentialOpts{
		Name:           data.Name.ValueString(),
		Host:           data.Host.ValueString(),
		Port:           data.Port.ValueInt64(),
		Username:       data.Username.ValueString(),
		PrivateKeyID:   privateKeyID,
		RemoteHostKey:  data.RemoteHostKey.ValueString(),
		ConnectTimeout: data.ConnectTimeout.ValueInt64(),
	}

	if !scan {
		return opts, true
	}

	hostKey, err := r.services.KeychainCredential.ScanRemoteHostKey(ctx, services.ScanRemoteHostKeyOpts{
		Host:           opts.Host,
		Port:           opts.Port,
		ConnectTimeout: opts.ConnectTimeout,
	})
	if err != nil {
		diags.AddError(
			"Unable to Discover Remote Host Key",
			fmt.Sprintf("Unable to scan the host key of %s: %s. Set remote_host_key to trust a known key instead.", opts.Host, err.Error()),
		)
		return opts, false
	}

	opts.RemoteHostKey = hostKey
	return opts, true
}

// mapSSHCredentialToModel maps a typed SSHCredential to the resource model.
func mapSSHCredentialToModel(credential *services.SSHCredential, data *SSHCredentialResourceModel) {
	data.ID = types.StringValue(strconv.FormatInt(credential.ID, 10))
	data.Name = types.StringValue(credential.Name)
	data.Host = types.StringValue(credential.Host)
	data.Port = types.Int64Value(credential.Port)
	data.Username = types.StringValue(credential.Username)
	data.PrivateKeyID = types.StringValue(strconv.FormatInt(credential.PrivateKeyID, 10))
	data.RemoteHostKey = types.StringValue(credential.RemoteHostKey)
	data.ConnectTimeout = types.Int64Value(credential.ConnectTimeout)
}

// parseKeychainCredentialID converts the string ID Terraform carries to the
// integer ID the keychaincredential.* endpoints take.
func parseKeychainCredentialID(id types.String, diags *diag.Diagnostics) (int64, bool) {
	parsed, err := strconv.ParseInt(id.ValueString(), 10, 64)
	if err != nil {
		diags.AddError(
			"Invalid ID",
			fmt.Sprintf("Unable to parse ID %q: %s", id.ValueString(), err.Error()),
		)
		return 0, false
	}
	return parsed, true
}
