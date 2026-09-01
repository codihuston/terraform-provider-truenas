package resources

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	truenastypes "github.com/deevus/terraform-provider-truenas/internal/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &APIKeyResource{}
	_ resource.ResourceWithConfigure   = &APIKeyResource{}
	_ resource.ResourceWithImportState = &APIKeyResource{}
)

// APIKeyResourceModel describes the resource data model.
type APIKeyResourceModel struct {
	ID             types.String                      `tfsdk:"id"`
	Name           types.String                      `tfsdk:"name"`
	Username       types.String                      `tfsdk:"username"`
	ExpiresAt      truenastypes.TimestampStringValue `tfsdk:"expires_at"`
	StoreKey       types.Bool                        `tfsdk:"store_key"`
	Key            types.String                      `tfsdk:"key"`
	UserIdentifier types.String                      `tfsdk:"user_identifier"`
	CreatedAt      types.String                      `tfsdk:"created_at"`
	Local          types.Bool                        `tfsdk:"local"`
	Revoked        types.Bool                        `tfsdk:"revoked"`
	RevokedReason  types.String                      `tfsdk:"revoked_reason"`
}

// APIKeyResource defines the resource implementation.
type APIKeyResource struct {
	BaseResource
}

// NewAPIKeyResource creates a new APIKeyResource.
func NewAPIKeyResource() resource.Resource {
	return &APIKeyResource{}
}

func (r *APIKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *APIKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an API key for a TrueNAS user account. TrueNAS discloses the key secret " +
			"only in the reply that creates it and never reads it back, so `key` is null for an " +
			"imported key, and for one created with `store_key = false` once it has been refreshed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "API key ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Human-readable name for the API key. Must be unique.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 200),
				},
			},
			"username": schema.StringAttribute{
				Description: "Username the key authenticates as. Changing this forces a new API key, " +
					"because TrueNAS cannot reassign an existing key to another account.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				Description: "RFC 3339 timestamp at which the key stops authenticating, for example " +
					"`2035-01-02T15:04:05Z`. Omit for a key that never expires.",
				Optional:   true,
				CustomType: truenastypes.TimestampStringType{},
			},
			"store_key": schema.BoolAttribute{
				Description: "Whether to persist `key` in Terraform state. When `false`, the secret is " +
					"readable only by resources in the same apply that creates the key, and is dropped " +
					"from state on the next refresh; nothing can recover it afterwards. Setting this " +
					"back to `true` forces a new API key, since that is the only way to obtain a secret. " +
					"For a key created with `store_key` set to `false`, this value must be known at plan time.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplaceIf(
						requiresReplaceWhenStoringKeyAgain,
						"If store_key is changed from false to true, Terraform will destroy and recreate the API key.",
						"If `store_key` is changed from `false` to `true`, Terraform will destroy and recreate the API key.",
					),
				},
			},
			"key": schema.StringAttribute{
				Description: "The API key secret, in the `<id>-<token>` form TrueNAS clients expect. " +
					"Null unless this resource created the key with `store_key` set to `true`.",
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					// The API issues no secret outside creation, so an update
					// carries forward whatever state holds.
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_identifier": schema.StringAttribute{
				Description: "UID of the local account, or SID of the directory account, that owns the key.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "RFC 3339 timestamp, in UTC, at which the key was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"local": schema.BoolAttribute{
				Description: "Whether the key belongs to a local account rather than a directory account.",
				Computed:    true,
			},
			"revoked": schema.BoolAttribute{
				Description: "Whether the key has been revoked and no longer authenticates. TrueNAS " +
					"revokes keys on its own, for example once they expire.",
				Computed: true,
			},
			"revoked_reason": schema.StringAttribute{
				Description: "Why TrueNAS revoked the key, or null while it is still valid.",
				Computed:    true,
			},
		},
	}
}

// requiresReplaceWhenStoringKeyAgain replaces the key when the plan switches
// key storage back on: the secret exists only in the reply to a creation, so
// an existing key can never start being stored. While storage is already on,
// nothing replaces, so an unknown plan is harmless. While it is off, an unknown
// plan is an error, because neither guess is safe. Otherwise the planned value
// decides, since it already carries the schema default for a config that omits
// store_key.
func requiresReplaceWhenStoringKeyAgain(ctx context.Context, req planmodifier.BoolRequest, resp *boolplanmodifier.RequiresReplaceIfFuncResponse) {
	if req.StateValue.ValueBool() {
		resp.RequiresReplace = false
		return
	}

	if req.PlanValue.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("store_key"),
			"Unknown store_key Value",
			"This API key was created with store_key set to false, so its secret is no longer in state. "+
				"Whether storing the key again requires issuing a new one cannot be decided while store_key "+
				"is unknown at plan time: assuming it stays false would leave a key whose secret can never be "+
				"recovered, and assuming it becomes true would destroy a key that is still valid. "+
				"Set store_key to a value that is known at plan time.",
		)
		return
	}

	resp.RequiresReplace = req.PlanValue.ValueBool()
}

func (r *APIKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data APIKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	expiresAt, err := data.ExpiresAt.TimePointer()
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("expires_at"), "Invalid Expiry Timestamp", err.Error())
		return
	}

	apiKey, err := r.services.APIKey.Create(ctx, services.CreateAPIKeyOpts{
		Name:      data.Name.ValueString(),
		Username:  data.Username.ValueString(),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create API Key",
			fmt.Sprintf("Unable to create API key: %s", err.Error()),
		)
		return
	}

	if apiKey == nil {
		resp.Diagnostics.AddError(
			"API Key Not Found",
			"API key was created but could not be found.",
		)
		return
	}

	mapAPIKeyToModel(apiKey, &data)

	// The secret is written to state even when store_key is false, because
	// that is what resources later in this apply read it from. Read drops it
	// again on the next refresh.
	data.Key = types.StringValue(apiKey.Key)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *APIKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data APIKeyResourceModel

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

	apiKey, err := r.services.APIKey.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read API Key",
			fmt.Sprintf("Unable to query API key: %s", err.Error()),
		)
		return
	}

	if apiKey == nil {
		// Key was deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	// An imported key has no store_key in state; it takes the schema default so
	// the first plan after the import is empty.
	if data.StoreKey.IsNull() {
		data.StoreKey = types.BoolValue(true)
	}

	mapAPIKeyToModel(apiKey, &data)

	// The reply never carries the secret, so the value already in state is the
	// only copy there will ever be. Drop it once the user has opted out.
	if !data.StoreKey.ValueBool() {
		data.Key = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *APIKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state APIKeyResourceModel
	var plan APIKeyResourceModel

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

	expiresAt, err := plan.ExpiresAt.TimePointer()
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("expires_at"), "Invalid Expiry Timestamp", err.Error())
		return
	}

	apiKey, err := r.services.APIKey.Update(ctx, id, services.UpdateAPIKeyOpts{
		Name:      plan.Name.ValueString(),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update API Key",
			fmt.Sprintf("Unable to update API key: %s", err.Error()),
		)
		return
	}

	if apiKey == nil {
		resp.Diagnostics.AddError(
			"API Key Not Found",
			"API key was updated but could not be found.",
		)
		return
	}

	// The reply carries no secret, and mapAPIKeyToModel never writes Key, so
	// the planned value the key attribute carried forward from state stands.
	mapAPIKeyToModel(apiKey, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *APIKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data APIKeyResourceModel

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

	if err := r.services.APIKey.Delete(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete API Key",
			fmt.Sprintf("Unable to delete API key: %s", err.Error()),
		)
		return
	}
}

// mapAPIKeyToModel maps a typed APIKey to the resource model, leaving the
// secret untouched because no API reply but the creation carries one.
func mapAPIKeyToModel(apiKey *services.APIKey, data *APIKeyResourceModel) {
	data.ID = types.StringValue(strconv.FormatInt(apiKey.ID, 10))
	data.Name = types.StringValue(apiKey.Name)
	data.Username = types.StringPointerValue(apiKey.Username)
	data.UserIdentifier = types.StringValue(apiKey.UserIdentifier)
	data.CreatedAt = types.StringValue(apiKey.CreatedAt.UTC().Format(time.RFC3339Nano))
	data.ExpiresAt = truenastypes.NewTimestampStringPointerValue(apiKey.ExpiresAt)
	data.Local = types.BoolValue(apiKey.Local)
	data.Revoked = types.BoolValue(apiKey.Revoked)
	data.RevokedReason = types.StringPointerValue(apiKey.RevokedReason)
}
