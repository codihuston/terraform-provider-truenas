package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// optionalString converts an optional string attribute to a pointer, so an
// unset attribute reaches the API as null rather than an empty string.
func optionalString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}

	s := v.ValueString()
	return &s
}

// stringPointerValue converts an API string pointer back to an attribute value.
func stringPointerValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// optionalInt64 converts an optional int64 attribute to a pointer, mapping
// null/unknown to nil so the API receives an explicit null.
func optionalInt64(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}

	n := v.ValueInt64()
	return &n
}

// int64PointerValue converts an API int64 pointer back to an attribute value.
func int64PointerValue(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

// stringsFromList reads a list attribute into a string slice, appending any
// conversion errors to diags. Null and unknown lists yield an empty slice.
func stringsFromList(ctx context.Context, list types.List, diags *diag.Diagnostics) []string {
	if list.IsNull() || list.IsUnknown() {
		return []string{}
	}

	items := []string{}
	diags.Append(list.ElementsAs(ctx, &items, false)...)
	return items
}

// listFromStrings converts a string slice from the API into a list attribute.
// Every element is a known string, so the conversion cannot fail.
func listFromStrings(values []string) types.List {
	elements := make([]attr.Value, len(values))
	for i, v := range values {
		elements[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elements)
}
