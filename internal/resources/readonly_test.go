package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// A read-only attribute is shown, never set: Computed without Optional, and
// without UseStateForUnknown, since Exchange may change it with any write.
func TestReadOnlyFieldSchema(t *testing.T) {
	for _, f := range []field{
		readOnlyAttr(stringAttr("s", "S", "d")),
		readOnlyAttr(enumAttr("e", "E", "d", "A", "B")),
		readOnlyAttr(boolAttr("b", "B", "d")),
		readOnlyAttr(setAttr("t", "T", "d")),
		readOnlyAttr(listAttr("l", "L", "d")),
		readOnlyAttr(int64Attr("i", "I", "d")),
	} {
		var computed, optional bool
		var modifiers int
		switch a := f.schemaAttribute().(type) {
		case schema.StringAttribute:
			computed, optional, modifiers = a.Computed, a.Optional, len(a.PlanModifiers)
		case schema.BoolAttribute:
			computed, optional, modifiers = a.Computed, a.Optional, len(a.PlanModifiers)
		case schema.SetAttribute:
			computed, optional, modifiers = a.Computed, a.Optional, len(a.PlanModifiers)
		case schema.ListAttribute:
			computed, optional, modifiers = a.Computed, a.Optional, len(a.PlanModifiers)
		case schema.Int64Attribute:
			computed, optional, modifiers = a.Computed, a.Optional, len(a.PlanModifiers)
		default:
			t.Fatalf("%s: unexpected attribute type %T", f.name, a)
		}
		if !computed || optional || modifiers != 0 {
			t.Errorf("%s: computed=%v optional=%v plan modifiers=%d, want computed only without modifiers",
				f.name, computed, optional, modifiers)
		}
	}
}

// Even if a value reached the configuration, a read-only attribute is never
// sent to Exchange.
func TestReadOnlyFieldNotWritten(t *testing.T) {
	r := &objectResource{spec: objectSpec{typeName: "x", noun: "X", adopted: true, fields: []field{
		stringAttr("internal_url", "InternalUrl", "d"),
		readOnlyAttr(stringAttr("shown", "Shown", "d")),
	}}}
	values := map[string]string{"name": "EX01", "internal_url": "https://mail.contoso.com/owa", "shown": "Require"}
	plan := mustObject(t, r, values)
	got, diags := r.writeValues(context.Background(), "Update", plan, plan)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if _, ok := got["Shown"]; ok {
		t.Errorf("read-only Shown was sent: %v", got)
	}
	if got["InternalUrl"] != "https://mail.contoso.com/owa" {
		t.Errorf("InternalUrl = %v, want the configured URL", got["InternalUrl"])
	}
}
