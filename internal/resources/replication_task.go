package resources

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &ReplicationTaskResource{}
	_ resource.ResourceWithConfigure      = &ReplicationTaskResource{}
	_ resource.ResourceWithImportState    = &ReplicationTaskResource{}
	_ resource.ResourceWithValidateConfig = &ReplicationTaskResource{}
)

// Transports the resource supports. The API also accepts LOCAL and SSH+NETCAT;
// see replicationTransportValidators for how a new transport is added.
var replicationSupportedTransports = []string{services.ReplicationTransportSSH}

// Directions the resource supports. The API also accepts PULL.
var replicationSupportedDirections = []string{services.ReplicationDirectionPush}

// ReplicationTaskScheduleModel describes the `schedule` block.
type ReplicationTaskScheduleModel struct {
	Minute types.String `tfsdk:"minute"`
	Hour   types.String `tfsdk:"hour"`
	Dom    types.String `tfsdk:"dom"`
	Month  types.String `tfsdk:"month"`
	Dow    types.String `tfsdk:"dow"`
	Begin  types.String `tfsdk:"begin"`
	End    types.String `tfsdk:"end"`
}

// ReplicationTaskResourceModel describes the resource data model.
type ReplicationTaskResourceModel struct {
	ID                      types.String                  `tfsdk:"id"`
	Name                    types.String                  `tfsdk:"name"`
	Direction               types.String                  `tfsdk:"direction"`
	Transport               types.String                  `tfsdk:"transport"`
	SSHCredentials          types.Int64                   `tfsdk:"ssh_credentials"`
	Sudo                    types.Bool                    `tfsdk:"sudo"`
	SourceDatasets          types.List                    `tfsdk:"source_datasets"`
	TargetDataset           types.String                  `tfsdk:"target_dataset"`
	Recursive               types.Bool                    `tfsdk:"recursive"`
	Exclude                 types.List                    `tfsdk:"exclude"`
	AlsoIncludeNamingSchema types.List                    `tfsdk:"also_include_naming_schema"`
	Auto                    types.Bool                    `tfsdk:"auto"`
	RetentionPolicy         types.String                  `tfsdk:"retention_policy"`
	LifetimeValue           types.Int64                   `tfsdk:"lifetime_value"`
	LifetimeUnit            types.String                  `tfsdk:"lifetime_unit"`
	Readonly                types.String                  `tfsdk:"readonly"`
	AllowFromScratch        types.Bool                    `tfsdk:"allow_from_scratch"`
	Compression             types.String                  `tfsdk:"compression"`
	SpeedLimit              types.Int64                   `tfsdk:"speed_limit"`
	Retries                 types.Int64                   `tfsdk:"retries"`
	LoggingLevel            types.String                  `tfsdk:"logging_level"`
	Enabled                 types.Bool                    `tfsdk:"enabled"`
	State                   types.String                  `tfsdk:"state"`
	Schedule                *ReplicationTaskScheduleModel `tfsdk:"schedule"`
}

// ReplicationTaskResource defines the resource implementation.
type ReplicationTaskResource struct {
	BaseResource
}

// NewReplicationTaskResource creates a new ReplicationTaskResource.
func NewReplicationTaskResource() resource.Resource {
	return &ReplicationTaskResource{}
}

func (r *ReplicationTaskResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_replication_task"
}

func (r *ReplicationTaskResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a ZFS replication task that pushes snapshots to a remote host over SSH. " +
			"Snapshots are not created by this resource: bind the task to a naming schema that matches " +
			"snapshots produced elsewhere, for example by a periodic snapshot task.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Replication task ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the replication task.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"direction": schema.StringAttribute{
				Description: "Direction of the transfer. Only `PUSH` is supported.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(services.ReplicationDirectionPush),
				Validators: []validator.String{
					stringvalidator.OneOf(replicationSupportedDirections...),
				},
			},
			"transport": schema.StringAttribute{
				Description: "Method of snapshot transfer. Only `SSH` is supported.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(services.ReplicationTransportSSH),
				Validators: []validator.String{
					stringvalidator.OneOf(replicationSupportedTransports...),
				},
			},
			"ssh_credentials": schema.Int64Attribute{
				Description: "ID of the `SSH_CREDENTIALS` keychain credential identifying the target host. " +
					"Required for the `SSH` transport.",
				Optional: true,
			},
			"sudo": schema.BoolAttribute{
				Description: "Run `zfs` through passwordless sudo on the target host. " +
					"Needed when the credential's user is not root.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"source_datasets": schema.ListAttribute{
				Description: "Datasets to replicate snapshots from, without a leading `/mnt`.",
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
			},
			"target_dataset": schema.StringAttribute{
				Description: "Dataset on the target host to put snapshots into, without a leading `/mnt`.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"recursive": schema.BoolAttribute{
				Description: "Replicate child datasets of every source dataset.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"exclude": schema.ListAttribute{
				Description: "Child datasets to exclude from replication. Requires `recursive`.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(emptyStringList()),
			},
			"also_include_naming_schema": schema.ListAttribute{
				Description: "`strftime` patterns identifying the snapshots to replicate, " +
					"for example `auto-%Y-%m-%d_%H-%M`.",
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
			},
			"auto": schema.BoolAttribute{
				Description: "Whether the task runs by itself. Derived from `schedule`: a task with a " +
					"schedule runs automatically, one without it only runs when triggered manually.",
				Computed: true,
			},
			"retention_policy": schema.StringAttribute{
				Description: "How snapshots are deleted on the target. `SOURCE` deletes snapshots that no " +
					"longer exist on the source, `CUSTOM` deletes snapshots older than " +
					"`lifetime_value`/`lifetime_unit`, and `NONE` never deletes.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						services.ReplicationRetentionSource,
						services.ReplicationRetentionCustom,
						services.ReplicationRetentionNone,
					),
				},
			},
			"lifetime_value": schema.Int64Attribute{
				Description: "How many `lifetime_unit`s to keep snapshots for. " +
					"Required by, and only valid with, the `CUSTOM` retention policy.",
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
					int64validator.AlsoRequires(path.MatchRoot("lifetime_unit")),
				},
			},
			"lifetime_unit": schema.StringAttribute{
				Description: "Unit `lifetime_value` counts. " +
					"Required by, and only valid with, the `CUSTOM` retention policy.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("HOUR", "DAY", "WEEK", "MONTH", "YEAR"),
					stringvalidator.AlsoRequires(path.MatchRoot("lifetime_value")),
				},
			},
			"readonly": schema.StringAttribute{
				Description: "How the target datasets' `readonly` property is handled. `SET` marks them " +
					"read-only after each run, `REQUIRE` refuses to run unless they already are, and " +
					"`IGNORE` leaves the property alone.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("SET"),
				Validators: []validator.String{
					stringvalidator.OneOf("SET", "REQUIRE", "IGNORE"),
				},
			},
			"allow_from_scratch": schema.BoolAttribute{
				Description: "Destroy every snapshot on the target and replicate from scratch when no " +
					"target snapshot matches the source.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"compression": schema.StringAttribute{
				Description: "Compress the SSH stream. Only valid for the `SSH` transport.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("LZ4", "PIGZ", "PLZIP"),
				},
			},
			"speed_limit": schema.Int64Attribute{
				Description: "Limit the SSH stream to this many bytes per second. " +
					"Only valid for the `SSH` transport.",
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"retries": schema.Int64Attribute{
				Description: "Number of attempts before a run is considered failed.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(5),
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"logging_level": schema.StringAttribute{
				Description: "Verbosity of the task's log.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("DEBUG", "INFO", "WARNING", "ERROR"),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "Enable the replication task. A disabled task neither runs on its schedule " +
					"nor can be triggered manually.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"state": schema.StringAttribute{
				Description: "Last known run state of the task, such as `PENDING`, `RUNNING`, " +
					"`FINISHED` or `ERROR`.",
				Computed: true,
			},
		},
		Blocks: map[string]schema.Block{
			"schedule": schema.SingleNestedBlock{
				Description: "When the task runs. Omit the block for a task that only runs when " +
					"triggered manually.",
				Attributes: map[string]schema.Attribute{
					"minute": schema.StringAttribute{
						Description: "Minute (0-59 or cron expression).",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("00"),
					},
					"hour": schema.StringAttribute{
						Description: "Hour (0-23 or cron expression).",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("*"),
					},
					"dom": schema.StringAttribute{
						Description: "Day of month (1-31 or cron expression).",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("*"),
					},
					"month": schema.StringAttribute{
						Description: "Month (1-12 or cron expression).",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("*"),
					},
					"dow": schema.StringAttribute{
						Description: "Day of week (1-7, Monday to Sunday, or cron expression).",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("*"),
					},
					"begin": schema.StringAttribute{
						Description: "Earliest time of day the task may start, in `HH:MM`.",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("00:00"),
					},
					"end": schema.StringAttribute{
						Description: "Latest time of day the task may start, in `HH:MM`.",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("23:59"),
					},
				},
			},
		},
	}
}

// ValidateConfig enforces the cross-field rules the API applies, so a bad
// configuration fails during plan rather than apply.
//
// Each rule lives in its own function keyed off the transport or the retention
// policy. Supporting a new transport means adding a case to
// validateReplicationTransport and leaving the existing branches untouched.
func (r *ReplicationTaskResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ReplicationTaskResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateReplicationTransport(&data, &resp.Diagnostics)
	validateReplicationRetention(&data, &resp.Diagnostics)
	validateReplicationExclude(&data, &resp.Diagnostics)
}

// validateReplicationTransport applies the rules specific to the configured
// transport. Unknown transports are left to the schema's OneOf validator.
func validateReplicationTransport(data *ReplicationTaskResourceModel, diags *diag.Diagnostics) {
	if data.Transport.IsUnknown() {
		return
	}

	switch transport := data.Transport.ValueString(); transport {
	case "", services.ReplicationTransportSSH:
		validateReplicationSSHTransport(data, diags)
	}
}

// validateReplicationSSHTransport requires the SSH credential that identifies
// the target host.
func validateReplicationSSHTransport(data *ReplicationTaskResourceModel, diags *diag.Diagnostics) {
	if data.SSHCredentials.IsNull() {
		diags.AddAttributeError(
			path.Root("ssh_credentials"),
			"Missing SSH Credentials",
			"The SSH transport requires ssh_credentials to identify the target host.",
		)
	}
}

// validateReplicationRetention ties the lifetime attributes to the CUSTOM
// retention policy, which is the only policy the API accepts them with.
func validateReplicationRetention(data *ReplicationTaskResourceModel, diags *diag.Diagnostics) {
	if data.RetentionPolicy.IsNull() || data.RetentionPolicy.IsUnknown() {
		return
	}

	custom := data.RetentionPolicy.ValueString() == services.ReplicationRetentionCustom
	set := !data.LifetimeValue.IsNull() || !data.LifetimeUnit.IsNull()

	switch {
	case custom && !set:
		diags.AddAttributeError(
			path.Root("lifetime_value"),
			"Missing Snapshot Lifetime",
			"The CUSTOM retention policy requires lifetime_value and lifetime_unit.",
		)
	case !custom && set:
		diags.AddAttributeError(
			path.Root("lifetime_value"),
			"Unexpected Snapshot Lifetime",
			fmt.Sprintf(
				"lifetime_value and lifetime_unit only apply to the CUSTOM retention policy, not %s.",
				data.RetentionPolicy.ValueString(),
			),
		)
	}
}

// validateReplicationExclude rejects exclusions on a non-recursive task, which
// the API refuses because there are no child datasets to exclude.
func validateReplicationExclude(data *ReplicationTaskResourceModel, diags *diag.Diagnostics) {
	if data.Exclude.IsNull() || data.Exclude.IsUnknown() || len(data.Exclude.Elements()) == 0 {
		return
	}
	if data.Recursive.IsUnknown() || data.Recursive.ValueBool() {
		return
	}

	diags.AddAttributeError(
		path.Root("exclude"),
		"Exclusions Require Recursive Replication",
		"Excluding child datasets is only supported when recursive is true.",
	)
}

// buildReplicationTaskOpts builds typed options from the resource model.
func buildReplicationTaskOpts(ctx context.Context, data *ReplicationTaskResourceModel) (services.CreateReplicationTaskOpts, diag.Diagnostics) {
	var diags diag.Diagnostics

	schedule := replicationScheduleFromModel(data.Schedule)

	opts := services.CreateReplicationTaskOpts{
		Name:             data.Name.ValueString(),
		Direction:        data.Direction.ValueString(),
		Transport:        data.Transport.ValueString(),
		SSHCredentials:   optionalInt64(data.SSHCredentials),
		Sudo:             data.Sudo.ValueBool(),
		TargetDataset:    data.TargetDataset.ValueString(),
		Recursive:        data.Recursive.ValueBool(),
		Auto:             schedule != nil,
		Schedule:         schedule,
		RetentionPolicy:  data.RetentionPolicy.ValueString(),
		LifetimeValue:    optionalInt64(data.LifetimeValue),
		LifetimeUnit:     optionalString(data.LifetimeUnit),
		Readonly:         data.Readonly.ValueString(),
		AllowFromScratch: data.AllowFromScratch.ValueBool(),
		Compression:      optionalString(data.Compression),
		SpeedLimit:       optionalInt64(data.SpeedLimit),
		Retries:          data.Retries.ValueInt64(),
		LoggingLevel:     optionalString(data.LoggingLevel),
		Enabled:          data.Enabled.ValueBool(),
	}

	opts.SourceDatasets = stringsFromList(ctx, data.SourceDatasets, &diags)
	opts.Exclude = stringsFromList(ctx, data.Exclude, &diags)
	opts.AlsoIncludeNamingSchema = stringsFromList(ctx, data.AlsoIncludeNamingSchema, &diags)

	return opts, diags
}

// replicationScheduleFromModel converts the schedule block to the service type.
// Attributes the practitioner left out carry the API's documented defaults from
// the schema, so every field is known here.
func replicationScheduleFromModel(block *ReplicationTaskScheduleModel) *services.ReplicationSchedule {
	if block == nil {
		return nil
	}

	return &services.ReplicationSchedule{
		Minute: block.Minute.ValueString(),
		Hour:   block.Hour.ValueString(),
		Dom:    block.Dom.ValueString(),
		Month:  block.Month.ValueString(),
		Dow:    block.Dow.ValueString(),
		Begin:  block.Begin.ValueString(),
		End:    block.End.ValueString(),
	}
}

func (r *ReplicationTaskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ReplicationTaskResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, diags := buildReplicationTaskOpts(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	task, err := r.services.Replication.Create(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Replication Task",
			fmt.Sprintf("Unable to create replication task: %s", err.Error()),
		)
		return
	}

	if task == nil {
		resp.Diagnostics.AddError(
			"Replication Task Not Found",
			"Replication task was created but could not be found.",
		)
		return
	}

	mapReplicationTaskToModel(task, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ReplicationTaskResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ReplicationTaskResourceModel

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

	task, err := r.services.Replication.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Replication Task",
			fmt.Sprintf("Unable to query replication task: %s", err.Error()),
		)
		return
	}

	if task == nil {
		// Replication task was deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	importing := replicationImportPending(ctx, req.Private, &resp.Diagnostics)
	if importing && resp.Private != nil {
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, replicationImportKey, nil)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(replicationTaskScopeDiagnostics(task, importing)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mapReplicationTaskToModel(task, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ReplicationTaskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state ReplicationTaskResourceModel
	var plan ReplicationTaskResourceModel

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

	opts, diags := buildReplicationTaskOpts(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	task, err := r.services.Replication.Update(ctx, id, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update Replication Task",
			fmt.Sprintf("Unable to update replication task: %s", err.Error()),
		)
		return
	}

	if task == nil {
		resp.Diagnostics.AddError(
			"Replication Task Not Found",
			"Replication task was updated but could not be found.",
		)
		return
	}

	mapReplicationTaskToModel(task, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ReplicationTaskResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ReplicationTaskResourceModel

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

	if err := r.services.Replication.Delete(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete Replication Task",
			fmt.Sprintf("Unable to delete replication task: %s", err.Error()),
		)
		return
	}
}

// replicationImportKey marks the state written by ImportState, so the Read the
// framework runs straight afterwards can tell an import from a refresh.
const replicationImportKey = "replication_task_import"

// ImportState seeds the ID and records that the read which follows is an
// import, so a task outside this resource's scope is refused rather than
// adopted. BaseResource.ImportState does the ID passthrough alone.
func (r *ReplicationTaskResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

	resp.Diagnostics.Append(resp.Private.SetKey(ctx, replicationImportKey, []byte(`{"pending":true}`))...)
}

// privateStateReader reads provider private state. The framework's concrete
// type is internal to it, so the read side is taken through this interface.
type privateStateReader interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

// replicationImportPending reports whether ImportState marked this read as the
// one that follows an import.
func replicationImportPending(ctx context.Context, private privateStateReader, diags *diag.Diagnostics) bool {
	marker, markerDiags := private.GetKey(ctx, replicationImportKey)
	diags.Append(markerDiags...)

	return len(marker) > 0
}

// replicationTaskScopeDiagnostics reports the ways a task falls outside the
// modes this resource manages. Membership is tested against the supported
// slices, so widening either one is all a future direction or transport needs.
//
// An import is refused outright: adopting the task would only let the next plan
// rewrite it. A refresh of a task already under management warns instead, so a
// task somebody flipped out of scope on the server can still be planned and,
// above all, destroyed.
func replicationTaskScopeDiagnostics(task *services.ReplicationTask, importing bool) diag.Diagnostics {
	var diags diag.Diagnostics

	add := func(summary, field, value string) {
		detail := fmt.Sprintf(
			"Replication task %d uses %s %q, which truenas_replication_task does not manage. "+
				"This resource currently supports only the SSH push case; see the resource "+
				"documentation for the modes that are out of scope.",
			task.ID, field, value,
		)

		if importing {
			diags.AddError(summary, detail)
			return
		}

		diags.AddWarning(summary, detail+" The task is kept in Terraform state so it can still be "+
			"planned and destroyed, but applying this configuration may propose rewriting it.")
	}

	if !slices.Contains(replicationSupportedDirections, task.Direction) {
		add("Unsupported Replication Task Direction", "direction", task.Direction)
	}

	if !slices.Contains(replicationSupportedTransports, task.Transport) {
		add("Unsupported Replication Task Transport", "transport", task.Transport)
	}

	return diags
}

// mapReplicationTaskToModel maps a typed ReplicationTask to the resource model.
func mapReplicationTaskToModel(task *services.ReplicationTask, data *ReplicationTaskResourceModel) {
	data.ID = types.StringValue(strconv.FormatInt(task.ID, 10))
	data.Name = types.StringValue(task.Name)
	data.Direction = types.StringValue(task.Direction)
	data.Transport = types.StringValue(task.Transport)
	data.SSHCredentials = int64PointerValue(task.SSHCredentials)
	data.Sudo = types.BoolValue(task.Sudo)
	data.SourceDatasets = listFromStrings(task.SourceDatasets)
	data.TargetDataset = types.StringValue(task.TargetDataset)
	data.Recursive = types.BoolValue(task.Recursive)
	data.Exclude = listFromStrings(task.Exclude)
	data.AlsoIncludeNamingSchema = listFromStrings(task.AlsoIncludeNamingSchema)
	data.Auto = types.BoolValue(task.Auto)
	data.RetentionPolicy = types.StringValue(task.RetentionPolicy)
	data.LifetimeValue = int64PointerValue(task.LifetimeValue)
	data.LifetimeUnit = stringPointerValue(task.LifetimeUnit)
	data.Readonly = types.StringValue(task.Readonly)
	data.AllowFromScratch = types.BoolValue(task.AllowFromScratch)
	data.Compression = stringPointerValue(task.Compression)
	data.SpeedLimit = int64PointerValue(task.SpeedLimit)
	data.Retries = types.Int64Value(task.Retries)
	data.LoggingLevel = stringPointerValue(task.LoggingLevel)
	data.Enabled = types.BoolValue(task.Enabled)
	data.State = types.StringValue(task.State)

	if task.Schedule == nil {
		data.Schedule = nil
		return
	}

	data.Schedule = &ReplicationTaskScheduleModel{
		Minute: types.StringValue(task.Schedule.Minute),
		Hour:   types.StringValue(task.Schedule.Hour),
		Dom:    types.StringValue(task.Schedule.Dom),
		Month:  types.StringValue(task.Schedule.Month),
		Dow:    types.StringValue(task.Schedule.Dow),
		Begin:  types.StringValue(task.Schedule.Begin),
		End:    types.StringValue(task.Schedule.End),
	}
}
