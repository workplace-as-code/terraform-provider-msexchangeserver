package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// nullPlan returns an empty plan for r: a destroy, until attributes are set.
func nullPlan(t *testing.T, r resource.Resource) tfsdk.Plan {
	t.Helper()
	ctx := context.Background()
	var s resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &s)
	if s.Diagnostics.HasError() {
		t.Fatal(s.Diagnostics)
	}
	return tfsdk.Plan{Schema: s.Schema, Raw: tftypes.NewValue(s.Schema.Type().TerraformType(ctx), nil)}
}

func TestEdgeRoleRefusesOrganizationResources(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		role    string
		edge    bool
		destroy bool
		wantErr bool
	}{
		{"organization resource on Edge", client.RoleEdge, false, false, true},
		{"destroy is always allowed", client.RoleEdge, false, true, false},
		{"Edge resource on Edge", client.RoleEdge, true, false, false},
		{"organization resource on Mailbox", client.RoleMailbox, false, false, false},
	} {
		r := &identityResource{Resource: &fakeResource{}, edge: tc.edge}
		r.Configure(ctx, resource.ConfigureRequest{ProviderData: client.NewForRole(tc.role)}, &resource.ConfigureResponse{})
		plan := nullPlan(t, r)
		if !tc.destroy {
			plan.SetAttribute(ctx, path.Root("name"), "contoso")
		}
		resp := resource.ModifyPlanResponse{Plan: plan}
		r.ModifyPlan(ctx, resource.ModifyPlanRequest{Plan: plan, State: nullState(plan)}, &resp)
		if got := hasError(resp, "Not available on an Edge Transport server"); got != tc.wantErr || len(resp.Diagnostics) > 1 {
			t.Errorf("%s: error = %v, want %v (%v)", tc.name, got, tc.wantErr, resp.Diagnostics)
		}
	}
}

func TestEveryOrganizationResourceIsRefusedOnEdge(t *testing.T) {
	edge := map[string]bool{}
	for _, reg := range registrations() {
		edge[typeName(context.Background(), reg.newResource().Metadata)] = reg.edge
	}
	for _, name := range []string{"msexchangeserver_transport_rule", "msexchangeserver_accepted_domain",
		"msexchangeserver_remote_domain", "msexchangeserver_transport_config"} {
		if !edge[name] {
			t.Errorf("%s should be available on Edge Transport servers", name)
		}
	}
	for _, name := range []string{"msexchangeserver_owa_mailbox_policy", "msexchangeserver_organization_config",
		"msexchangeserver_retention_policy", "msexchangeserver_email_address_policy"} {
		if edge[name] {
			t.Errorf("%s should not be available on Edge Transport servers", name)
		}
	}
}

func TestQuarantineNeedsAnEdgeServer(t *testing.T) {
	ctx := context.Background()
	for role, wantErr := range map[string]bool{client.RoleMailbox: true, client.RoleEdge: false} {
		r := &transportRuleResource{client: client.NewForRole(role)}
		plan := nullPlan(t, r)
		plan.SetAttribute(ctx, path.Root("name"), "Spam")
		plan.SetAttribute(ctx, path.Root("action"), "Quarantine")
		resp := resource.ModifyPlanResponse{Plan: plan}
		r.ModifyPlan(ctx, resource.ModifyPlanRequest{Plan: plan, State: nullState(plan)}, &resp)
		if got := hasError(resp, "Quarantine needs an Edge Transport server"); got != wantErr || len(resp.Diagnostics) > 1 {
			t.Errorf("role %s: error = %v, want %v (%v)", role, got, wantErr, resp.Diagnostics)
		}
	}
}

func TestExecutableAttachmentNeedsAMailboxServer(t *testing.T) {
	ctx := context.Background()
	for role, wantErr := range map[string]bool{client.RoleMailbox: false, client.RoleEdge: true} {
		r := &transportRuleResource{client: client.NewForRole(role)}
		plan := nullPlan(t, r)
		plan.SetAttribute(ctx, path.Root("name"), "Executables")
		plan.SetAttribute(ctx, path.Root("action"), "Reject")
		plan.SetAttribute(ctx, path.Root("attachment_is_executable"), true)
		resp := resource.ModifyPlanResponse{Plan: plan}
		r.ModifyPlan(ctx, resource.ModifyPlanRequest{Plan: plan, State: nullState(plan)}, &resp)
		if got := hasError(resp, "Not available on an Edge Transport server"); got != wantErr || len(resp.Diagnostics) > 1 {
			t.Errorf("role %s: error = %v, want %v (%v)", role, got, wantErr, resp.Diagnostics)
		}
	}
}

// nullState is the state of a resource being created.
func nullState(plan tfsdk.Plan) tfsdk.State {
	return tfsdk.State{Schema: plan.Schema, Raw: tftypes.NewValue(plan.Raw.Type(), nil)}
}

// hasError reports whether the plan was refused with the given summary.
func hasError(resp resource.ModifyPlanResponse, summary string) bool {
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == summary {
			return true
		}
	}
	return false
}
