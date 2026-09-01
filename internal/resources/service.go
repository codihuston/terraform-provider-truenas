package resources

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &ServiceResource{}
	_ resource.ResourceWithConfigure   = &ServiceResource{}
	_ resource.ResourceWithImportState = &ServiceResource{}
)

// ServiceResourceModel describes the resource data model.
type ServiceResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Enable  types.Bool   `tfsdk:"enable"`
	Running types.Bool   `tfsdk:"running"`
}

// ServiceResource defines the resource implementation.
type ServiceResource struct {
	BaseResource
}

// NewServiceResource creates a new ServiceResource.
func NewServiceResource() resource.Resource {
	return &ServiceResource{}
}

func (r *ServiceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *ServiceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the state of a built-in TrueNAS system service, such as NFS or SMB. " +
			"Services are part of the appliance and are never created or removed: this resource only " +
			"takes ownership of an existing service's boot and run state. Destroying the resource " +
			"therefore stops the service and disables it at boot.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Service name, which is also the identifier used for import.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the service to manage, as reported by the `service` API " +
					"namespace — for example `nfs`, `cifs`, `ssh` or `iscsitarget`. The set of " +
					"services depends on the TrueNAS version and edition; an unknown name fails " +
					"with the list the appliance does offer.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enable": schema.BoolAttribute{
				Description: "Start the service automatically on boot.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"running": schema.BoolAttribute{
				Description: "Run the service now. Terraform starts or stops the service to match " +
					"this value, and reports it back from the server so an out-of-band stop shows " +
					"up as drift.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
		},
	}
}

// ImportState adopts an existing service by name. The name is the identifier,
// so both `id` and `name` are seeded from it and Read fills in the rest.
func (r *ServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

func (r *ServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.reconcile(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()

	svc, err := r.services.Service.Get(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Service",
			fmt.Sprintf("Unable to query service %q: %s", name, err.Error()),
		)
		return
	}

	if svc == nil {
		// The appliance no longer offers this service, for example after an
		// edition or major version change.
		resp.State.RemoveResource(ctx)
		return
	}

	mapServiceToModel(svc, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServiceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.reconcile(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete releases the service by stopping it and disabling it at boot. The
// service itself belongs to the appliance and cannot be removed.
func (r *ServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()

	svc, err := r.services.Service.Get(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Service",
			fmt.Sprintf("Unable to query service %q: %s", name, err.Error()),
		)
		return
	}

	if svc == nil {
		return
	}

	if svc.Running() {
		if err := r.services.Service.Stop(ctx, name); err != nil {
			resp.Diagnostics.AddError(
				"Unable to Stop Service",
				fmt.Sprintf("Unable to stop service %q: %s", name, err.Error()),
			)
			return
		}
	}

	if svc.Enable {
		if err := r.services.Service.SetEnable(ctx, name, false); err != nil {
			resp.Diagnostics.AddError(
				"Unable to Disable Service",
				fmt.Sprintf("Unable to disable service %q: %s", name, err.Error()),
			)
		}
	}
}

// reconcile drives the live service towards the desired boot and run state in
// data, then overwrites data with the state the server reports back. Terraform
// rejects an applied value that disagrees with a known plan, so a server that
// refuses the requested state is reported as an actionable error instead.
func (r *ServiceResource) reconcile(ctx context.Context, data *ServiceResourceModel, diags *diag.Diagnostics) {
	name := data.Name.ValueString()

	svc, err := r.services.Service.Get(ctx, name)
	if err != nil {
		diags.AddError(
			"Unable to Read Service",
			fmt.Sprintf("Unable to query service %q: %s", name, err.Error()),
		)
		return
	}

	if svc == nil {
		diags.AddError(
			"Unknown Service",
			fmt.Sprintf("TrueNAS does not offer a service named %q.%s", name, r.knownServiceNames(ctx)),
		)
		return
	}

	if enable := data.Enable.ValueBool(); enable != svc.Enable {
		if err := r.services.Service.SetEnable(ctx, name, enable); err != nil {
			diags.AddError(
				"Unable to Update Service",
				fmt.Sprintf("Unable to set enable=%t on service %q: %s", enable, name, err.Error()),
			)
			return
		}
	}

	if running := data.Running.ValueBool(); running != svc.Running() {
		if err := r.setRunning(ctx, name, running); err != nil {
			diags.AddError(
				"Unable to Control Service",
				fmt.Sprintf("Unable to %s service %q: %s", startOrStop(running), name, err.Error()),
			)
			return
		}
	}

	// Re-read rather than assume: a service can refuse to stay up without its
	// start call failing, and that belongs in state as drift, not as a lie.
	svc, err = r.services.Service.Get(ctx, name)
	if err != nil {
		diags.AddError(
			"Unable to Read Service",
			fmt.Sprintf("Unable to query service %q: %s", name, err.Error()),
		)
		return
	}

	if svc == nil {
		diags.AddError(
			"Service Not Found",
			fmt.Sprintf("Service %q disappeared while it was being configured.", name),
		)
		return
	}

	mismatch := false

	if enable := data.Enable.ValueBool(); enable != svc.Enable {
		mismatch = true
		diags.AddError(
			"Service Enable Not Applied",
			fmt.Sprintf("Service %q reports enable=%t after its update call succeeded, but the configuration asks for enable=%t.",
				name, svc.Enable, enable),
		)
	}

	if running := data.Running.ValueBool(); running != svc.Running() {
		mismatch = true
		if running {
			diags.AddError(
				"Service Not Running",
				fmt.Sprintf("Service %q did not stay running after its start call succeeded. Check the service's logs on the appliance for why it stopped.", name),
			)
		} else {
			diags.AddError(
				"Service Still Running",
				fmt.Sprintf("Service %q is still running after its stop call succeeded. Check the service's logs on the appliance for why it stayed up.", name),
			)
		}
	}

	if mismatch {
		return
	}

	mapServiceToModel(svc, data)
}

// setRunning starts or stops the service to match running.
func (r *ServiceResource) setRunning(ctx context.Context, name string, running bool) error {
	if running {
		return r.services.Service.Start(ctx, name)
	}
	return r.services.Service.Stop(ctx, name)
}

// knownServiceNames renders a trailing sentence listing the services the
// appliance offers, so a typo is fixable without leaving the error. It stays
// silent when the follow-up query fails: the caller already has a real error.
func (r *ServiceResource) knownServiceNames(ctx context.Context) string {
	svcs, err := r.services.Service.List(ctx)
	if err != nil || len(svcs) == 0 {
		return ""
	}

	names := make([]string, len(svcs))
	for i, svc := range svcs {
		names[i] = svc.Name
	}
	sort.Strings(names)

	return fmt.Sprintf(" Known services: %s.", strings.Join(names, ", "))
}

// startOrStop names the operation for an error message.
func startOrStop(running bool) string {
	if running {
		return "start"
	}
	return "stop"
}

// mapServiceToModel maps a typed SystemService to the resource model.
func mapServiceToModel(svc *services.SystemService, data *ServiceResourceModel) {
	data.ID = types.StringValue(svc.Name)
	data.Name = types.StringValue(svc.Name)
	data.Enable = types.BoolValue(svc.Enable)
	data.Running = types.BoolValue(svc.Running())
}
