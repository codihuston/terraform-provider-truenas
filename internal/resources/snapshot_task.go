package resources

import (
	"context"
	"fmt"
	"strconv"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.Resource                = &SnapshotTaskResource{}
	_ resource.ResourceWithConfigure   = &SnapshotTaskResource{}
	_ resource.ResourceWithImportState = &SnapshotTaskResource{}
)

// SnapshotTaskResourceModel describes the resource data model.
type SnapshotTaskResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Dataset       types.String `tfsdk:"dataset"`
	Recursive     types.Bool   `tfsdk:"recursive"`
	Exclude       types.List   `tfsdk:"exclude"`
	LifetimeValue types.Int64  `tfsdk:"lifetime_value"`
	LifetimeUnit  types.String `tfsdk:"lifetime_unit"`
	NamingSchema  types.String `tfsdk:"naming_schema"`
	AllowEmpty    types.Bool   `tfsdk:"allow_empty"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	Schedule      types.Object `tfsdk:"schedule"`
	VMwareSync    types.Bool   `tfsdk:"vmware_sync"`
}

// SnapshotTaskResource defines the resource implementation.
type SnapshotTaskResource struct {
	BaseResource
}

// NewSnapshotTaskResource creates a new SnapshotTaskResource.
func NewSnapshotTaskResource() resource.Resource {
	return &SnapshotTaskResource{}
}

func (r *SnapshotTaskResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot_task"
}

// snapshotTaskScheduleAttrTypes mirrors the `schedule` attribute so the object
// can be built and read back without re-deriving its type.
var snapshotTaskScheduleAttrTypes = map[string]attr.Type{
	"minute": types.StringType,
	"hour":   types.StringType,
	"dom":    types.StringType,
	"month":  types.StringType,
	"dow":    types.StringType,
	"begin":  types.StringType,
	"end":    types.StringType,
}

// snapshotTaskScheduleDefaults reproduces the API's own schedule defaults, so
// omitting `schedule` yields the same task TrueNAS would have created.
var snapshotTaskScheduleDefaults = services.SnapshotSchedule{
	Minute: "00",
	Hour:   "*",
	Dom:    "*",
	Month:  "*",
	Dow:    "*",
	Begin:  "00:00",
	End:    "23:59",
}

// snapshotTaskScheduleObject converts a schedule to its attribute value. Every
// field is a known string, so the conversion cannot fail.
func snapshotTaskScheduleObject(schedule services.SnapshotSchedule) types.Object {
	return types.ObjectValueMust(snapshotTaskScheduleAttrTypes, map[string]attr.Value{
		"minute": types.StringValue(schedule.Minute),
		"hour":   types.StringValue(schedule.Hour),
		"dom":    types.StringValue(schedule.Dom),
		"month":  types.StringValue(schedule.Month),
		"dow":    types.StringValue(schedule.Dow),
		"begin":  types.StringValue(schedule.Begin),
		"end":    types.StringValue(schedule.End),
	})
}

func (r *SnapshotTaskResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a periodic snapshot task, which takes ZFS snapshots of a dataset on a " +
			"cron schedule and expires them once their lifetime elapses.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Periodic snapshot task ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dataset": schema.StringAttribute{
				Description: "Dataset to snapshot, as a ZFS name such as \"tank/data\" " +
					"(not a mount point).",
				Required: true,
			},
			"recursive": schema.BoolAttribute{
				Description: "Also snapshot the child datasets of `dataset`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"exclude": schema.ListAttribute{
				Description: "Child datasets to leave out of a recursive snapshot. " +
					"Only valid when `recursive` is true.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			},
			"lifetime_value": schema.Int64Attribute{
				Description: "Number of `lifetime_unit` periods to keep snapshots for before " +
					"they are eligible for deletion.",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(2),
			},
			"lifetime_unit": schema.StringAttribute{
				Description: "Unit `lifetime_value` is counted in. " +
					"Valid values: HOUR, DAY, WEEK, MONTH, YEAR.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("WEEK"),
				Validators: []validator.String{
					stringvalidator.OneOf("HOUR", "DAY", "WEEK", "MONTH", "YEAR"),
				},
			},
			"naming_schema": schema.StringAttribute{
				Description: "strftime pattern used to name the snapshots this task takes. " +
					"It must produce a unique name for every run, so it has to include " +
					"enough time fields to distinguish them.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("auto-%Y-%m-%d_%H-%M"),
			},
			"allow_empty": schema.BoolAttribute{
				Description: "Take a snapshot even when nothing changed since the previous run.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"enabled": schema.BoolAttribute{
				Description: "Enable or disable the task.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"schedule": schema.SingleNestedAttribute{
				Description: "Cron schedule the task runs on. Omit it to snapshot hourly, " +
					"on the hour, every day.",
				Optional: true,
				Computed: true,
				Default:  objectdefault.StaticValue(snapshotTaskScheduleObject(snapshotTaskScheduleDefaults)),
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
						Description: "Day of week, 1 (Monday) to 7 (Sunday), or a cron expression.",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("*"),
					},
					"begin": schema.StringAttribute{
						Description: "Start of the daily window in which the task may run, as \"HH:MM\". " +
							"A begin later than end defines a window that wraps past midnight.",
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString("00:00"),
					},
					"end": schema.StringAttribute{
						Description: "End of the daily window in which the task may run, as \"HH:MM\". " +
							"An end earlier than begin defines a window that wraps past midnight.",
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString("23:59"),
					},
				},
			},
			"vmware_sync": schema.BoolAttribute{
				Description: "Whether VMware virtual machines are synchronised before snapshots " +
					"are taken. Derived from the VMware-snapshot configuration, not settable here.",
				Computed: true,
			},
		},
	}
}

// snapshotTaskScheduleModel is the tfsdk shape of the `schedule` attribute.
type snapshotTaskScheduleModel struct {
	Minute types.String `tfsdk:"minute"`
	Hour   types.String `tfsdk:"hour"`
	Dom    types.String `tfsdk:"dom"`
	Month  types.String `tfsdk:"month"`
	Dow    types.String `tfsdk:"dow"`
	Begin  types.String `tfsdk:"begin"`
	End    types.String `tfsdk:"end"`
}

// buildSnapshotTaskOpts builds typed options from the resource model.
func buildSnapshotTaskOpts(ctx context.Context, data *SnapshotTaskResourceModel) (services.CreateSnapshotTaskOpts, diag.Diagnostics) {
	var diags diag.Diagnostics

	opts := services.CreateSnapshotTaskOpts{
		Dataset:       data.Dataset.ValueString(),
		Recursive:     data.Recursive.ValueBool(),
		LifetimeValue: data.LifetimeValue.ValueInt64(),
		LifetimeUnit:  data.LifetimeUnit.ValueString(),
		NamingSchema:  data.NamingSchema.ValueString(),
		AllowEmpty:    data.AllowEmpty.ValueBool(),
		Enabled:       data.Enabled.ValueBool(),
		Schedule:      snapshotTaskScheduleFromObject(ctx, data.Schedule, &diags),
	}

	opts.Exclude = snapshotTaskStringsFromList(ctx, data.Exclude, &diags)

	return opts, diags
}

func (r *SnapshotTaskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SnapshotTaskResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, diags := buildSnapshotTaskOpts(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	task, err := r.services.SnapshotTask.Create(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Periodic Snapshot Task",
			fmt.Sprintf("Unable to create periodic snapshot task: %s", err.Error()),
		)
		return
	}

	if task == nil {
		resp.Diagnostics.AddError(
			"Periodic Snapshot Task Not Found",
			"Periodic snapshot task was created but could not be found.",
		)
		return
	}

	mapSnapshotTaskToModel(task, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SnapshotTaskResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SnapshotTaskResourceModel

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

	task, err := r.services.SnapshotTask.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Periodic Snapshot Task",
			fmt.Sprintf("Unable to query periodic snapshot task: %s", err.Error()),
		)
		return
	}

	if task == nil {
		// Task was deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	mapSnapshotTaskToModel(task, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SnapshotTaskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state SnapshotTaskResourceModel
	var plan SnapshotTaskResourceModel

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

	opts, diags := buildSnapshotTaskOpts(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	task, err := r.services.SnapshotTask.Update(ctx, id, opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update Periodic Snapshot Task",
			fmt.Sprintf("Unable to update periodic snapshot task: %s", err.Error()),
		)
		return
	}

	if task == nil {
		resp.Diagnostics.AddError(
			"Periodic Snapshot Task Not Found",
			"Periodic snapshot task was updated but could not be found.",
		)
		return
	}

	mapSnapshotTaskToModel(task, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SnapshotTaskResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SnapshotTaskResourceModel

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

	if err := r.services.SnapshotTask.Delete(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete Periodic Snapshot Task",
			fmt.Sprintf("Unable to delete periodic snapshot task: %s", err.Error()),
		)
		return
	}
}

// mapSnapshotTaskToModel maps a typed SnapshotTask to the resource model.
func mapSnapshotTaskToModel(task *services.SnapshotTask, data *SnapshotTaskResourceModel) {
	data.ID = types.StringValue(strconv.FormatInt(task.ID, 10))
	data.Dataset = types.StringValue(task.Dataset)
	data.Recursive = types.BoolValue(task.Recursive)
	data.Exclude = snapshotTaskListFromStrings(task.Exclude)
	data.LifetimeValue = types.Int64Value(task.LifetimeValue)
	data.LifetimeUnit = types.StringValue(task.LifetimeUnit)
	data.NamingSchema = types.StringValue(task.NamingSchema)
	data.AllowEmpty = types.BoolValue(task.AllowEmpty)
	data.Enabled = types.BoolValue(task.Enabled)
	data.Schedule = snapshotTaskScheduleObject(task.Schedule)
	data.VMwareSync = types.BoolValue(task.VMwareSync)
}

// snapshotTaskScheduleFromObject reads the schedule attribute into the typed
// service struct. A null or unknown object yields the API's own defaults, which
// is what the schema default puts in the plan anyway.
func snapshotTaskScheduleFromObject(ctx context.Context, obj types.Object, diags *diag.Diagnostics) services.SnapshotSchedule {
	if obj.IsNull() || obj.IsUnknown() {
		return snapshotTaskScheduleDefaults
	}

	var model snapshotTaskScheduleModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return snapshotTaskScheduleDefaults
	}

	return services.SnapshotSchedule{
		Minute: model.Minute.ValueString(),
		Hour:   model.Hour.ValueString(),
		Dom:    model.Dom.ValueString(),
		Month:  model.Month.ValueString(),
		Dow:    model.Dow.ValueString(),
		Begin:  model.Begin.ValueString(),
		End:    model.End.ValueString(),
	}
}

// snapshotTaskStringsFromList reads a list attribute into a string slice,
// appending any conversion errors to diags. Null and unknown lists yield an
// empty slice.
func snapshotTaskStringsFromList(ctx context.Context, list types.List, diags *diag.Diagnostics) []string {
	if list.IsNull() || list.IsUnknown() {
		return []string{}
	}

	items := []string{}
	diags.Append(list.ElementsAs(ctx, &items, false)...)
	return items
}

// snapshotTaskListFromStrings converts a string slice from the API into a list
// attribute. Every element is a known string, so the conversion cannot fail.
func snapshotTaskListFromStrings(values []string) types.List {
	elements := make([]attr.Value, len(values))
	for i, v := range values {
		elements[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elements)
}
