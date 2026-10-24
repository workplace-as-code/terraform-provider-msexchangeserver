package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// Compile-time checks that transportRuleResource implements the interfaces we rely on.
var (
	_ resource.Resource                   = &transportRuleResource{}
	_ resource.ResourceWithImportState    = &transportRuleResource{}
	_ resource.ResourceWithValidateConfig = &transportRuleResource{}
	_ resource.ResourceWithModifyPlan     = &transportRuleResource{}
)

// transportRuleActions lists the actions the bridge script implements.
// Quarantine is an Edge Transport action: Mailbox servers reject the parameter
// (ModifyPlan refuses it unless the provider role is Edge).
var transportRuleActions = []string{"Reject", "Quarantine", "MoveToJmf"}

// transportRuleResource is the main resource struct
type transportRuleResource struct {
	client *client.Client
}

// transportRuleModel represents the state of a transport rule in HCL/tfstate
type transportRuleModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Priority               types.Int64  `tfsdk:"priority"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	Action                 types.String `tfsdk:"action"`
	RejectMessage          types.String `tfsdk:"reject_message"`
	FromAddressContains    types.List   `tfsdk:"from_address_contains"`
	SubjectContains        types.List   `tfsdk:"subject_contains"`
	AttachmentIsExecutable types.Bool   `tfsdk:"attachment_is_executable"`
}

// transportRuleAPIResponse represents the JSON response from the PowerShell script
type transportRuleAPIResponse struct {
	Identity               string   `json:"Identity"`
	Name                   string   `json:"Name"`
	Priority               int64    `json:"Priority"`
	Enabled                bool     `json:"Enabled"`
	Action                 string   `json:"Action"`
	RejectMessage          string   `json:"RejectMessage"`
	FromAddressContains    []string `json:"FromAddressContains"`
	SubjectContains        []string `json:"SubjectContains"`
	AttachmentIsExecutable bool     `json:"AttachmentIsExecutable"`
}

// NewTransportRuleResource is the factory function registered in the provider
func NewTransportRuleResource() resource.Resource {
	return &transportRuleResource{}
}

// Metadata defines the resource name as used in HCL
func (r *transportRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_transport_rule"
}

// Schema defines all resource attributes
func (r *transportRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange transport rule (mail flow rule). At least one condition is required.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "The unique identifier of the transport rule.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				MarkdownDescription: "The name of the transport rule. Changing it renames the rule in place.",
			},
			"priority": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				// When not configured, keep whatever Exchange assigned instead of
				// forcing 0 (which would push every other rule down and cause drift).
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				MarkdownDescription: "The priority of the rule. Lower numbers are evaluated first. When omitted, Exchange assigns the lowest priority.",
			},
			"enabled": optionalBool("Whether the rule is enabled. A new rule is enabled unless this is `false`."),
			"action": schema.StringAttribute{
				Required:   true,
				Validators: []validator.String{stringvalidator.OneOf(transportRuleActions...)},
				MarkdownDescription: "The action to take when conditions are met. Valid values: `Reject`, `MoveToJmf` (deliver to the Junk Email folder), `Quarantine` (Edge Transport servers only, " +
					"to the spam quarantine mailbox). On a Mailbox server, `Reject` returns a non-delivery report with `reject_message`; on an Edge Transport server, " +
					"it rejects the message during the SMTP session (code 550) with `reject_message` as the response text.",
			},
			"reject_message": optionalString("The rejection message shown to senders. Required when `action` is `Reject`."),
			"from_address_contains": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Condition: the sender address contains any of these strings.",
			},
			"subject_contains": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Condition: the subject contains any of these words or phrases.",
			},
			"attachment_is_executable": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Condition: an attachment contains executable content.",
			},
		},
	}
}

// ValidateConfig runs at plan time. It blocks configurations that would be
// dangerous or that the bridge cannot apply, before anything touches Exchange.
func (r *transportRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg transportRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Values that come from other resources are unknown until apply; skip the
	// check rather than reject a config that may turn out valid.
	if cfg.FromAddressContains.IsUnknown() || cfg.SubjectContains.IsUnknown() || cfg.AttachmentIsExecutable.IsUnknown() {
		return
	}
	hasCondition := len(cfg.FromAddressContains.Elements()) > 0 ||
		len(cfg.SubjectContains.Elements()) > 0 ||
		cfg.AttachmentIsExecutable.ValueBool()
	if !hasCondition {
		resp.Diagnostics.AddAttributeError(path.Root("action"), "Transport rule without condition",
			"Set at least one of from_address_contains, subject_contains or attachment_is_executable. "+
				"A transport rule without conditions applies to every message in the organization.")
	}

	if cfg.Action.ValueString() == "Reject" && !cfg.RejectMessage.IsUnknown() && cfg.RejectMessage.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("reject_message"), "Missing reject_message",
			"reject_message is required when action is \"Reject\".")
	}
}

// Configure retrieves the shared client injected by the provider
func (r *transportRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	apiClient, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = apiClient
}

// writeParams converts the plan into bridge parameters for Create and Update.
func (r *transportRuleResource) writeParams(ctx context.Context, action string, plan transportRuleModel) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	from, d := encodeStringList(ctx, plan.FromAddressContains)
	diags.Append(d...)
	subject, d := encodeStringList(ctx, plan.SubjectContains)
	diags.Append(d...)

	params := map[string]string{
		"Action":                 action,
		"Name":                   plan.Name.ValueString(),
		"RuleAction":             plan.Action.ValueString(),
		"RejectMessage":          plan.RejectMessage.ValueString(),
		"FromAddressContains":    from,
		"SubjectContains":        subject,
		"AttachmentIsExecutable": fmt.Sprintf("%t", plan.AttachmentIsExecutable.ValueBool()),
	}
	// Unknown means "not configured": Exchange keeps its value or picks one.
	putBool(params, "Enabled", plan.Enabled)
	if !plan.Priority.IsUnknown() && !plan.Priority.IsNull() {
		params["Priority"] = fmt.Sprintf("%d", plan.Priority.ValueInt64())
	}
	return params, diags
}

// Create handles `terraform apply` for a new resource
func (r *transportRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan transportRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params, diags := r.writeParams(ctx, "Create", plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "transport_rule.ps1", params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create transport rule", err.Error())
		return
	}

	var apiResp transportRuleAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(applyTransportRuleResponse(&plan, apiResp)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read fetches the real state from Exchange — used for drift detection
func (r *transportRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state transportRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "transport_rule.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read transport rule", err.Error())
		return
	}
	if client.IsNotFound(output) {
		// The bridge returned nothing: the rule was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	var apiResp transportRuleAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(applyTransportRuleResponse(&state, apiResp)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update handles modifications — triggered when the plan differs from the state
func (r *transportRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state transportRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params, diags := r.writeParams(ctx, "Update", plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	params["Identity"] = state.ID.ValueString()

	output, err := r.client.RunScript(ctx, "transport_rule.ps1", params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update transport rule", err.Error())
		return
	}

	// Store what Exchange actually applied, not just what was planned.
	var apiResp transportRuleAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(applyTransportRuleResponse(&plan, apiResp)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the resource — triggered by `terraform destroy`
func (r *transportRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state transportRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "transport_rule.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete transport rule", err.Error())
	}
}

func (r *transportRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyTransportRuleResponse copies the bridge response into the model.
func applyTransportRuleResponse(m *transportRuleModel, a transportRuleAPIResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(a.Identity)
	m.Name = types.StringValue(a.Name)
	m.Priority = types.Int64Value(a.Priority)
	m.Enabled = types.BoolValue(a.Enabled)
	m.Action = types.StringValue(a.Action)
	m.RejectMessage = types.StringValue(a.RejectMessage)

	var d diag.Diagnostics
	m.FromAddressContains, d = stringListOrNull(m.FromAddressContains, a.FromAddressContains)
	diags.Append(d...)
	m.SubjectContains, d = stringListOrNull(m.SubjectContains, a.SubjectContains)
	diags.Append(d...)

	// Keep null when unset in config and false in Exchange, to avoid a
	// perpetual null -> false diff.
	if a.AttachmentIsExecutable || !m.AttachmentIsExecutable.IsNull() {
		m.AttachmentIsExecutable = types.BoolValue(a.AttachmentIsExecutable)
	}
	return diags
}

// stringListOrNull returns a null list when Exchange has no values and the
// current value is null, so an unset optional list does not show a diff.
func stringListOrNull(current types.List, values []string) (types.List, diag.Diagnostics) {
	if len(values) == 0 && current.IsNull() {
		return types.ListNull(types.StringType), nil
	}
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValue(types.StringType, elems)
}

// encodeStringList encodes a list as base64(JSON) for the bridge, so values
// containing commas, quotes or spaces are passed safely on the command line.
// Null or empty lists encode as "".
func encodeStringList(ctx context.Context, l types.List) (string, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() || len(l.Elements()) == 0 {
		return "", nil
	}
	var items []string
	diags := l.ElementsAs(ctx, &items, false)
	if diags.HasError() {
		return "", diags
	}
	raw, err := json.Marshal(items)
	if err != nil {
		diags.AddError("Failed to encode list", err.Error())
		return "", diags
	}
	return base64.StdEncoding.EncodeToString(raw), diags
}

// ModifyPlan marks the id as unknown when the rule is renamed. Exchange uses
// the rule name as its identity, so a rename changes the id; announcing the old
// id in the plan would make Terraform reject the apply result as inconsistent.
func (r *transportRuleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Some conditions and actions only exist on one role (Set-TransportRule
	// documentation): refuse them at plan time rather than fail the apply.
	if !req.Plan.Raw.IsNull() && r.client != nil {
		var action types.String
		var executable types.Bool
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("action"), &action)...)
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("attachment_is_executable"), &executable)...)
		edge := r.client.Role() == client.RoleEdge
		if !edge && action.ValueString() == "Quarantine" {
			resp.Diagnostics.AddAttributeError(path.Root("action"), "Quarantine needs an Edge Transport server",
				"On Exchange Server, the Quarantine action only exists on Edge Transport servers. Use Reject or MoveToJmf, or a provider block whose role is \"Edge\".")
		}
		if edge && executable.ValueBool() {
			resp.Diagnostics.AddAttributeError(path.Root("attachment_is_executable"), "Not available on an Edge Transport server",
				"The executable attachment condition only exists on Mailbox servers. Edge Transport rules can match the subject or the sender address.")
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return // create or destroy: nothing to adjust
	}
	var plan, state transportRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Name.IsUnknown() && !plan.Name.Equal(state.Name) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())...)
	}
}
