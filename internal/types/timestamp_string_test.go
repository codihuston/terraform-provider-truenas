package types

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestTimestampStringType_Equal(t *testing.T) {
	if !(TimestampStringType{}).Equal(TimestampStringType{}) {
		t.Error("expected TimestampStringType to equal itself")
	}
	if (TimestampStringType{}).Equal(basetypes.StringType{}) {
		t.Error("expected TimestampStringType not to equal StringType")
	}
}

func TestTimestampStringType_String(t *testing.T) {
	if got := (TimestampStringType{}).String(); got != "TimestampStringType" {
		t.Errorf("expected 'TimestampStringType', got %q", got)
	}
}

func TestTimestampStringType_ValueType(t *testing.T) {
	if _, ok := (TimestampStringType{}).ValueType(context.Background()).(TimestampStringValue); !ok {
		t.Error("expected a TimestampStringValue")
	}
}

func TestTimestampStringType_ValueFromTerraform(t *testing.T) {
	got, err := (TimestampStringType{}).ValueFromTerraform(
		context.Background(),
		tftypes.NewValue(tftypes.String, "2030-01-02T03:04:05Z"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	value, ok := got.(TimestampStringValue)
	if !ok {
		t.Fatalf("expected TimestampStringValue, got %T", got)
	}
	if value.ValueString() != "2030-01-02T03:04:05Z" {
		t.Errorf("unexpected value %q", value.ValueString())
	}
}

func TestTimestampStringType_ValueFromTerraform_WrongType(t *testing.T) {
	_, err := (TimestampStringType{}).ValueFromTerraform(
		context.Background(),
		tftypes.NewValue(tftypes.Bool, true),
	)
	if err == nil {
		t.Fatal("expected error for a non-string value")
	}
}

func TestTimestampStringValue_Type(t *testing.T) {
	if _, ok := NewTimestampStringValue("2030-01-02T03:04:05Z").Type(context.Background()).(TimestampStringType); !ok {
		t.Error("expected a TimestampStringType")
	}
}

func TestTimestampStringValue_Equal(t *testing.T) {
	value := NewTimestampStringValue("2030-01-02T03:04:05Z")

	if !value.Equal(NewTimestampStringValue("2030-01-02T03:04:05Z")) {
		t.Error("expected identical values to be equal")
	}
	// Equal is textual: only StringSemanticEquals compares instants.
	if value.Equal(NewTimestampStringValue("2030-01-02T04:04:05+01:00")) {
		t.Error("expected differing text to be unequal")
	}
	if value.Equal(basetypes.NewStringValue("2030-01-02T03:04:05Z")) {
		t.Error("expected a plain StringValue to be unequal")
	}
}

func TestTimestampStringValue_ValidateAttribute(t *testing.T) {
	tests := map[string]struct {
		value   TimestampStringValue
		wantErr bool
	}{
		"utc":             {value: NewTimestampStringValue("2030-01-02T03:04:05Z")},
		"offset":          {value: NewTimestampStringValue("2030-01-02T04:04:05+01:00")},
		"fractional":      {value: NewTimestampStringValue("2030-01-02T03:04:05.5Z")},
		"milliseconds":    {value: NewTimestampStringValue("2030-01-02T03:04:05.123Z")},
		"sub-millisecond": {value: NewTimestampStringValue("2030-01-02T03:04:05.5001Z"), wantErr: true},
		"microseconds":    {value: NewTimestampStringValue("2030-01-02T03:04:05.000001Z"), wantErr: true},
		"null":            {value: NewTimestampStringNull()},
		"unknown":         {value: NewTimestampStringUnknown()},
		"date only":       {value: NewTimestampStringValue("2030-01-02"), wantErr: true},
		"no offset":       {value: NewTimestampStringValue("2030-01-02T03:04:05"), wantErr: true},
		"nonsensical":     {value: NewTimestampStringValue("tomorrow"), wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resp := &xattr.ValidateAttributeResponse{}
			tt.value.ValidateAttribute(
				context.Background(),
				xattr.ValidateAttributeRequest{Path: path.Root("expires_at")},
				resp,
			)

			if got := resp.Diagnostics.HasError(); got != tt.wantErr {
				t.Errorf("expected error %v, got diagnostics %v", tt.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestTimestampStringValue_StringSemanticEquals(t *testing.T) {
	tests := map[string]struct {
		old  TimestampStringValue
		new  TimestampStringValue
		want bool
	}{
		"same text":         {NewTimestampStringValue("2030-01-02T03:04:05Z"), NewTimestampStringValue("2030-01-02T03:04:05Z"), true},
		"same instant":      {NewTimestampStringValue("2030-01-02T04:04:05+01:00"), NewTimestampStringValue("2030-01-02T03:04:05Z"), true},
		"different instant": {NewTimestampStringValue("2030-01-02T03:04:05Z"), NewTimestampStringValue("2030-01-02T03:04:06Z"), false},
		"both null":         {NewTimestampStringNull(), NewTimestampStringNull(), true},
		"one null":          {NewTimestampStringNull(), NewTimestampStringValue("2030-01-02T03:04:05Z"), false},
		"one unknown":       {NewTimestampStringUnknown(), NewTimestampStringValue("2030-01-02T03:04:05Z"), false},
		"both unparseable":  {NewTimestampStringValue("later"), NewTimestampStringValue("later"), true},
		"one unparseable":   {NewTimestampStringValue("later"), NewTimestampStringValue("2030-01-02T03:04:05Z"), false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, diags := tt.old.StringSemanticEquals(context.Background(), tt.new)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestNewTimestampStringPointerValue(t *testing.T) {
	if got := NewTimestampStringPointerValue(nil); !got.IsNull() {
		t.Errorf("expected null for a nil time, got %q", got.ValueString())
	}

	// A non-UTC instant is rendered in UTC, matching how the API reports it.
	instant := time.Date(2030, time.January, 2, 4, 4, 5, 0, time.FixedZone("CET", 3600))
	if got := NewTimestampStringPointerValue(&instant); got.ValueString() != "2030-01-02T03:04:05Z" {
		t.Errorf("expected '2030-01-02T03:04:05Z', got %q", got.ValueString())
	}
}

func TestTimestampStringValue_TimePointer(t *testing.T) {
	if got, err := NewTimestampStringNull().TimePointer(); got != nil || err != nil {
		t.Errorf("expected nil, nil for a null value, got %v, %v", got, err)
	}
	if got, err := NewTimestampStringUnknown().TimePointer(); got != nil || err != nil {
		t.Errorf("expected nil, nil for an unknown value, got %v, %v", got, err)
	}

	got, err := NewTimestampStringValue("2030-01-02T04:04:05+01:00").TimePointer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC); !got.Equal(want) {
		t.Errorf("expected %s, got %s", want, got)
	}

	if _, err := NewTimestampStringValue("tomorrow").TimePointer(); err == nil {
		t.Error("expected a parse error")
	}
}
