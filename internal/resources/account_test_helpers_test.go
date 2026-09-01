package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// resourceSchema returns the schema of a resource under test.
func resourceSchema(t *testing.T, r resource.Resource) rschema.Schema {
	t.Helper()

	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", resp.Diagnostics)
	}

	return resp.Schema
}

// objectValue builds a value for the whole schema from the named attributes.
// Attributes that are not named are null, which is how Terraform represents an
// unset optional attribute and an as-yet-unresolved computed one in state.
func objectValue(t *testing.T, s rschema.Schema, attrs map[string]tftypes.Value) tftypes.Value {
	t.Helper()

	objectType, ok := s.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatalf("expected schema to be an object type, got %T", s.Type())
	}

	values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attrType := range objectType.AttributeTypes {
		if value, ok := attrs[name]; ok {
			values[name] = value
			continue
		}
		values[name] = tftypes.NewValue(attrType, nil)
	}

	for name := range attrs {
		if _, ok := objectType.AttributeTypes[name]; !ok {
			t.Fatalf("attribute %q is not in the schema", name)
		}
	}

	return tftypes.NewValue(objectType, values)
}

// stringList builds a list of string values for use in a schema value.
func stringList(items ...string) tftypes.Value {
	elements := make([]tftypes.Value, len(items))
	for i, item := range items {
		elements[i] = tftypes.NewValue(tftypes.String, item)
	}

	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, elements)
}

// int64Set builds a set of number values for use in a schema value.
func int64Set(items ...int64) tftypes.Value {
	elements := make([]tftypes.Value, len(items))
	for i, item := range items {
		elements[i] = tftypes.NewValue(tftypes.Number, item)
	}

	return tftypes.NewValue(tftypes.Set{ElementType: tftypes.Number}, elements)
}
