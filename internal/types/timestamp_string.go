package types

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Ensure interfaces are implemented.
var (
	_ basetypes.StringTypable                    = TimestampStringType{}
	_ basetypes.StringValuable                   = TimestampStringValue{}
	_ xattr.ValidateableAttribute                = TimestampStringValue{}
	_ basetypes.StringValuableWithSemanticEquals = TimestampStringValue{}
)

// TimestampStringType is a custom type for RFC 3339 timestamps. Values compare
// by the instant they denote rather than by their text, so a configured
// timestamp written in a local offset matches the UTC form the API returns.
type TimestampStringType struct {
	basetypes.StringType
}

// Equal returns true if the given type is equivalent.
func (t TimestampStringType) Equal(o attr.Type) bool {
	other, ok := o.(TimestampStringType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

// String returns a human-readable string of the type.
func (t TimestampStringType) String() string {
	return "TimestampStringType"
}

// ValueType returns the value type.
func (t TimestampStringType) ValueType(ctx context.Context) attr.Value {
	return TimestampStringValue{}
}

// ValueFromString converts a StringValue to a TimestampStringValue.
func (t TimestampStringType) ValueFromString(ctx context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return TimestampStringValue{StringValue: in}, nil
}

// ValueFromTerraform converts a tftypes.Value to a TimestampStringValue.
func (t TimestampStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}

	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}

	return stringValuable.(TimestampStringValue), nil
}

// TimestampStringValue is an RFC 3339 timestamp that compares by instant.
type TimestampStringValue struct {
	basetypes.StringValue
}

// Type returns the type of this value.
func (v TimestampStringValue) Type(ctx context.Context) attr.Type {
	return TimestampStringType{}
}

// Equal returns true if the values are equal (including null/unknown state).
func (v TimestampStringValue) Equal(o attr.Value) bool {
	other, ok := o.(TimestampStringValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

// ValidateAttribute reports a configured value that is not an RFC 3339
// timestamp, so the error surfaces during plan rather than as an API rejection.
func (v TimestampStringValue) ValidateAttribute(ctx context.Context, req xattr.ValidateAttributeRequest, resp *xattr.ValidateAttributeResponse) {
	if v.IsNull() || v.IsUnknown() {
		return
	}

	if _, err := time.Parse(time.RFC3339, v.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid RFC 3339 Timestamp",
			fmt.Sprintf("Expected an RFC 3339 timestamp such as \"2035-01-02T15:04:05Z\", got %q: %s", v.ValueString(), err),
		)
	}
}

// StringSemanticEquals compares two timestamps by the instant they denote.
// Values that do not parse fall back to a textual comparison; ValidateAttribute
// already rejects unparseable configuration.
func (v TimestampStringValue) StringSemanticEquals(ctx context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, d := newValuable.ToStringValue(ctx)
	diags.Append(d...)
	if diags.HasError() {
		return false, diags
	}

	if v.IsNull() || newValue.IsNull() || v.IsUnknown() || newValue.IsUnknown() {
		return v.StringValue.Equal(newValue), diags
	}

	oldTime, oldErr := time.Parse(time.RFC3339, v.ValueString())
	newTime, newErr := time.Parse(time.RFC3339, newValue.ValueString())
	if oldErr != nil || newErr != nil {
		return v.ValueString() == newValue.ValueString(), diags
	}

	return oldTime.Equal(newTime), diags
}

// NewTimestampStringValue creates a TimestampStringValue with the given string.
func NewTimestampStringValue(value string) TimestampStringValue {
	return TimestampStringValue{StringValue: basetypes.NewStringValue(value)}
}

// NewTimestampStringNull creates a null TimestampStringValue.
func NewTimestampStringNull() TimestampStringValue {
	return TimestampStringValue{StringValue: basetypes.NewStringNull()}
}

// NewTimestampStringUnknown creates an unknown TimestampStringValue.
func NewTimestampStringUnknown() TimestampStringValue {
	return TimestampStringValue{StringValue: basetypes.NewStringUnknown()}
}

// NewTimestampStringPointerValue creates a TimestampStringValue from a time
// pointer, rendering the instant in UTC. A nil time yields a null value.
func NewTimestampStringPointerValue(value *time.Time) TimestampStringValue {
	if value == nil {
		return NewTimestampStringNull()
	}
	return NewTimestampStringValue(value.UTC().Format(time.RFC3339Nano))
}

// TimePointer parses the value into a time pointer for sending to the API.
// Null and unknown values yield nil, which the API reads as "no expiration".
func (v TimestampStringValue) TimePointer() (*time.Time, error) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, v.ValueString())
	if err != nil {
		return nil, fmt.Errorf("parse timestamp %q: %w", v.ValueString(), err)
	}
	return &parsed, nil
}
