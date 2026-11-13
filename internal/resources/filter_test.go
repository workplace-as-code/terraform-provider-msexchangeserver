package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// filterTestResource is a resource with a recipient filter and a canonical
// name, independent of the provider's specs.
func filterTestResource() *objectResource {
	return &objectResource{spec: objectSpec{
		typeName: "filter_test",
		noun:     "AddressList",
		fields: []field{
			filterAttr("recipient_filter", "RecipientFilter", "d"),
			caselessAttr(stringAttr("recipient_container", "RecipientContainer", "d")),
		},
	}}
}

const (
	configuredFilter = "Company -eq 'Contoso' -and Department -eq 'Sales'"
	rewrittenFilter  = "((Company -eq 'Contoso') -and (Department -eq 'Sales'))"
)

func filterOutput(filter string) []byte {
	return []byte(`{"Name":"Sales","RecipientFilter":"` + filter + `","RecipientContainer":"contoso.com/Paris"}`)
}

// Exchange returns the text it returned after the last write: the configured
// text stays in the state, so the rewrite shows no diff.
func TestFilterKeepsConfiguredTextWhileExchangeReturnsRecorded(t *testing.T) {
	r := filterTestResource()
	prior := mustObject(t, r, map[string]string{"name": "Sales", "recipient_filter": configuredFilter})
	state, diags := r.fromAPI(prior, filterOutput(rewrittenFilter), map[string]string{"recipient_filter": rewrittenFilter})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got := state.Attributes()["recipient_filter"].(types.String).ValueString(); got != configuredFilter {
		t.Errorf("recipient_filter = %q, want the configured text", got)
	}
}

// A filter changed outside Terraform shows Exchange's text: the next plan
// writes the configured one back.
func TestFilterShowsOutsideChange(t *testing.T) {
	r := filterTestResource()
	prior := mustObject(t, r, map[string]string{"name": "Sales", "recipient_filter": configuredFilter})
	other := "((Department -eq 'Other'))"
	state, _ := r.fromAPI(prior, filterOutput(other), map[string]string{"recipient_filter": rewrittenFilter})
	if got := state.Attributes()["recipient_filter"].(types.String).ValueString(); got != other {
		t.Errorf("recipient_filter = %q, want Exchange's text %q", got, other)
	}
}

// After an import nothing is recorded: Exchange's text shows.
func TestFilterAfterImportShowsReadText(t *testing.T) {
	r := filterTestResource()
	prior := mustObject(t, r, map[string]string{"name": "Sales", "recipient_filter": configuredFilter})
	state, _ := r.fromAPI(prior, filterOutput(rewrittenFilter), nil)
	if got := state.Attributes()["recipient_filter"].(types.String).ValueString(); got != rewrittenFilter {
		t.Errorf("recipient_filter = %q, want Exchange's text", got)
	}
}

// filtersOf returns the filters of a bridge response, to record after a write.
func TestFiltersOfReturnsWrittenFilters(t *testing.T) {
	got := filterTestResource().filtersOf(filterOutput(rewrittenFilter))
	if len(got) != 1 || got["recipient_filter"] != rewrittenFilter {
		t.Errorf("filtersOf = %v", got)
	}
}

// A caseless string keeps the configured case when Exchange returns another one.
func TestCaselessKeepsConfiguredCase(t *testing.T) {
	r := filterTestResource()
	prior := mustObject(t, r, map[string]string{"name": "Sales", "recipient_container": "Contoso.com/PARIS"})
	state, _ := r.fromAPI(prior, filterOutput(rewrittenFilter), nil)
	if got := state.Attributes()["recipient_container"].(types.String).ValueString(); got != "Contoso.com/PARIS" {
		t.Errorf("recipient_container = %q, want the configured case", got)
	}
}

// trueOnlyAttr refuses false: Exchange cannot unset a default, another object
// must take it.
func TestTrueOnlyRefusesFalse(t *testing.T) {
	a, ok := trueOnlyAttr(boolAttr("is_default", "IsDefault", "d")).schemaAttribute().(schema.BoolAttribute)
	if !ok {
		t.Fatal("trueOnlyAttr is not a bool attribute")
	}
	check := func(v bool) bool {
		for _, val := range a.Validators {
			resp := &validator.BoolResponse{}
			val.ValidateBool(context.Background(), validator.BoolRequest{ConfigValue: types.BoolValue(v)}, resp)
			if resp.Diagnostics.HasError() {
				return true
			}
		}
		return false
	}
	if !check(false) {
		t.Error("false passes validation")
	}
	if check(true) {
		t.Error("true is refused")
	}
}

// planCheck runs on create and on update, never on a destroy, and its errors
// stop the plan.
func TestPlanCheckRunsOnCreateAndUpdate(t *testing.T) {
	var calls []bool
	spec := objectSpec{typeName: "x", noun: "X", fields: []field{boolAttr("b", "B", "d")},
		planCheck: func(config, plan, state map[string]attr.Value) diag.Diagnostics {
			calls = append(calls, state == nil)
			var d diag.Diagnostics
			d.AddError("refused", "refused")
			return d
		}}
	if !spec.hasPlanAdjustments() {
		t.Error("a spec with planCheck has no plan adjustments")
	}
	r := &objectResource{spec: spec}
	plan := map[string]attr.Value{"id": types.StringValue("a"), "name": types.StringValue("a"), "b": types.BoolValue(true)}
	if d := r.checkPlan(plan, plan, nil); !d.HasError() {
		t.Error("create: planCheck error not returned")
	}
	if d := r.checkPlan(plan, plan, plan); !d.HasError() {
		t.Error("update: planCheck error not returned")
	}
	if len(calls) != 2 || !calls[0] || calls[1] {
		t.Errorf("calls (state nil?) = %v, want [true false]", calls)
	}
}
