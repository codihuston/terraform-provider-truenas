package resources

import (
	"context"
	"fmt"
	"strconv"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &SSHKeypairResource{}
	_ resource.ResourceWithConfigure   = &SSHKeypairResource{}
	_ resource.ResourceWithImportState = &SSHKeypairResource{}
)

// SSHKeypairResourceModel describes the resource data model.
type SSHKeypairResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	PrivateKey      types.String `tfsdk:"private_key"`
	PrivateKeyWOVer types.Int64  `tfsdk:"private_key_wo_version"`
	PublicKey       types.String `tfsdk:"public_key"`
}

// SSHKeypairResource defines the resource implementation.
type SSHKeypairResource struct {
	BaseResource
}

// NewSSHKeypairResource creates a new SSHKeypairResource.
func NewSSHKeypairResource() resource.Resource {
	return &SSHKeypairResource{}
}

func (r *SSHKeypairResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_keypair"
}

func (r *SSHKeypairResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an SSH key pair in the TrueNAS keychain. A `truenas_ssh_credential` " +
			"authenticates with a key pair stored here rather than carrying the key itself.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Keychain credential ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name that distinguishes this key pair from others in the keychain.",
				Required:    true,
			},
			"private_key": schema.StringAttribute{
				Description: "SSH private key in OpenSSH format. This is a write-only attribute: it is " +
					"never written to state and never read back, so a key replaced outside " +
					"Terraform produces no drift. Increment `private_key_wo_version` to send it " +
					"again. TrueNAS derives `public_key` from it.",
				Required:  true,
				Sensitive: true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"private_key_wo_version": schema.Int64Attribute{
				Description: "Change this value to re-send `private_key` on the next apply. Because " +
					"`private_key` is write-only, this is the only way to rotate the key.",
				Required: true,
			},
			"public_key": schema.StringAttribute{
				Description: "Public half of the key pair, in OpenSSH format, as derived by TrueNAS. " +
					"Add it to the remote account's `authorized_keys` to let the connection " +
					"authenticate.",
				Computed: true,
			},
		},
	}
}

func (r *SSHKeypairResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SSHKeypairResourceModel
	var config SSHKeypairResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keypair, err := r.services.KeychainCredential.CreateSSHKeyPair(ctx, services.CreateSSHKeyPairOpts{
		Name:       data.Name.ValueString(),
		PrivateKey: config.PrivateKey.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create SSH Key Pair",
			fmt.Sprintf("Unable to create SSH key pair: %s", err.Error()),
		)
		return
	}

	mapSSHKeypairToModel(keypair, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSHKeypairResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SSHKeypairResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, ok := parseKeychainCredentialID(data.ID, &resp.Diagnostics)
	if !ok {
		return
	}

	keypair, err := r.services.KeychainCredential.GetSSHKeyPair(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read SSH Key Pair",
			fmt.Sprintf("Unable to query SSH key pair: %s", err.Error()),
		)
		return
	}

	if keypair == nil {
		// Key pair was deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	mapSSHKeypairToModel(keypair, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSHKeypairResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state SSHKeypairResourceModel
	var plan SSHKeypairResourceModel
	var config SSHKeypairResourceModel

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

	opts := services.UpdateSSHKeyPairOpts{Name: plan.Name.ValueString()}

	// The stored key is only replaced when the version says so: private_key is
	// write-only, so an unchanged version cannot be distinguished from a key
	// edited in place, and re-sending on every apply would invalidate the
	// public key a remote authorized_keys already trusts.
	if !plan.PrivateKeyWOVer.Equal(state.PrivateKeyWOVer) {
		privateKey := config.PrivateKey.ValueString()
		opts.PrivateKey = &privateKey
	}

	keypair, err := r.services.KeychainCredential.UpdateSSHKeyPair(ctx, id, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update SSH Key Pair",
			fmt.Sprintf("Unable to update SSH key pair: %s", err.Error()),
		)
		return
	}

	mapSSHKeypairToModel(keypair, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SSHKeypairResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SSHKeypairResourceModel

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
			"Unable to Delete SSH Key Pair",
			fmt.Sprintf("Unable to delete SSH key pair: %s", err.Error()),
		)
	}
}

// mapSSHKeypairToModel maps a typed SSHKeyPair to the resource model.
// private_key is write-only and so is deliberately left untouched.
func mapSSHKeypairToModel(keypair *services.SSHKeyPair, data *SSHKeypairResourceModel) {
	data.ID = types.StringValue(strconv.FormatInt(keypair.ID, 10))
	data.Name = types.StringValue(keypair.Name)
	data.PublicKey = types.StringValue(keypair.PublicKey)
}
