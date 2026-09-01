package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// remoteHostKeyPlanModifier returns a plan modifier for the computed
// `remote_host_key` attribute. It keeps the stored key while the connection
// stays put, and plans a fresh scan when the provider owns the key and the
// connection moves to another host or port.
func remoteHostKeyPlanModifier() planmodifier.String {
	return &remoteHostKeyModifier{}
}

type remoteHostKeyModifier struct{}

func (m *remoteHostKeyModifier) Description(ctx context.Context) string {
	return "Plans a fresh host key scan when the connection moves and remote_host_key is not configured."
}

func (m *remoteHostKeyModifier) MarkdownDescription(ctx context.Context) string {
	return "Plans a fresh host key scan when `host` or `port` changes and `remote_host_key` is not configured; otherwise keeps the stored value."
}

func (m *remoteHostKeyModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	// A configured host key belongs to the user and is never replaced by a scan.
	if !req.ConfigValue.IsNull() {
		return
	}

	var stateHost, planHost types.String
	var statePort, planPort types.Int64

	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("host"), &stateHost)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("host"), &planHost)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("port"), &statePort)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("port"), &planPort)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !planHost.Equal(stateHost) || !planPort.Equal(statePort) {
		resp.PlanValue = types.StringUnknown()
	}
}
