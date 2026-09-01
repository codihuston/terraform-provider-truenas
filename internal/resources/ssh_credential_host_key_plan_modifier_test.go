package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestRemoteHostKeyPlanModifier_Descriptions(t *testing.T) {
	m := remoteHostKeyPlanModifier()

	if m.Description(context.Background()) == "" {
		t.Error("expected non-empty description")
	}
	if m.MarkdownDescription(context.Background()) == "" {
		t.Error("expected non-empty markdown description")
	}
}

// The planned remote_host_key has to be unknown in exactly the cases where
// Update writes a freshly scanned key, or Terraform rejects the apply as an
// inconsistent result.
func TestRemoteHostKeyPlanModifier_PlanModifyString(t *testing.T) {
	movedHost := map[string]tftypes.Value{"host": tftypes.NewValue(tftypes.String, "moved.example.com")}
	movedPort := map[string]tftypes.Value{"port": tftypes.NewValue(tftypes.Number, 2022)}

	tests := []struct {
		name        string
		nullState   bool
		nullPlan    bool
		planned     map[string]tftypes.Value
		managed     bool
		wantUnknown bool
	}{
		{name: "create", nullState: true},
		{name: "destroy", nullPlan: true, planned: movedHost},
		{name: "host changed, key unmanaged", planned: movedHost, wantUnknown: true},
		{name: "port changed, key unmanaged", planned: movedPort, wantUnknown: true},
		{name: "host changed, key configured", planned: movedHost, managed: true},
		{name: "nothing moved, key unmanaged"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := resourceSchema(t, NewSSHCredentialResource())
			objectType := s.Type().TerraformType(context.Background())

			stateRaw := objectValue(t, s, testSSHCredentialAttrs())
			planRaw := objectValue(t, s, withAttrs(testSSHCredentialAttrs(), tc.planned))
			configRaw := objectValue(t, s, withAttrs(sshCredentialUnmanagedHostKeyAttrs(), tc.planned))

			configValue := types.StringNull()
			if tc.managed {
				configRaw = planRaw
				configValue = types.StringValue(testRemoteHostKey)
			}
			if tc.nullState {
				stateRaw = tftypes.NewValue(objectType, nil)
			}
			if tc.nullPlan {
				planRaw = tftypes.NewValue(objectType, nil)
			}

			stateValue := types.StringValue(testRemoteHostKey)
			req := planmodifier.StringRequest{
				Path:        path.Root("remote_host_key"),
				Config:      tfsdk.Config{Schema: s, Raw: configRaw},
				ConfigValue: configValue,
				Plan:        tfsdk.Plan{Schema: s, Raw: planRaw},
				PlanValue:   stateValue,
				State:       tfsdk.State{Schema: s, Raw: stateRaw},
				StateValue:  stateValue,
			}
			resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}

			remoteHostKeyPlanModifier().PlanModifyString(context.Background(), req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected errors: %v", resp.Diagnostics)
			}
			if got := resp.PlanValue.IsUnknown(); got != tc.wantUnknown {
				t.Fatalf("expected unknown plan value %v, got %v (%v)", tc.wantUnknown, got, resp.PlanValue)
			}
			if !tc.wantUnknown && !resp.PlanValue.Equal(stateValue) {
				t.Errorf("expected the stored host key to be kept, got %v", resp.PlanValue)
			}
		})
	}
}
