package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

var _ resource.Resource = &emailAddressPolicyResource{}
var _ resource.ResourceWithImportState = &emailAddressPolicyResource{}

type emailAddressPolicyResource struct{ client *client.Client }

type emailAddressPolicyModel struct {
	ID                           types.String `tfsdk:"id"`
	Name                         types.String `tfsdk:"name"`
	EnabledEmailAddressTemplates types.List   `tfsdk:"enabled_email_address_templates"`
	Priority                     types.Int64  `tfsdk:"priority"`
	IncludedRecipients           types.String `tfsdk:"included_recipients"`
	ConditionalDepartment        types.String `tfsdk:"conditional_department"`
	ConditionalCompany           types.String `tfsdk:"conditional_company"`
	// ApplyToRecipients is a provider setting, not an Exchange one: it is kept
	// from the configuration, never read from Exchange.
	ApplyToRecipients types.Bool `tfsdk:"apply_to_recipients"`
}

type emailAddressPolicyAPIResponse struct {
	Identity                     string          `json:"Identity"`
	Name                         string          `json:"Name"`
	EnabledEmailAddressTemplates FlexStringSlice `json:"EnabledEmailAddressTemplates"`
	Priority                     int64           `json:"Priority"`
	IncludedRecipients           string          `json:"IncludedRecipients"`
	ConditionalDepartment        string          `json:"ConditionalDepartment"`
	ConditionalCompany           string          `json:"ConditionalCompany"`
}

func NewEmailAddressPolicyResource() resource.Resource { return &emailAddressPolicyResource{} }

func (r *emailAddressPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_address_policy"
}

func (r *emailAddressPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an email address policy, which defines how email addresses are generated for recipients. Exchange applies a new or changed policy " +
			"to the recipients created or modified afterwards; existing recipients only get the new addresses when `Update-EmailAddressPolicy` runs, which " +
			"`apply_to_recipients` does.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{MarkdownDescription: "The unique identifier of the email address policy.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Name of the email address policy."},
			"enabled_email_address_templates": optionalList("SMTP address templates (e.g. `SMTP:%g.%s@contoso.com`). The primary address uses `SMTP:` (uppercase)."),
			"priority":                        optionalInt64("Priority of the policy. Lower numbers take precedence. The default policy always comes last and reads as `0`."),
			"included_recipients":             optionalString("Recipient types to apply this policy to. Valid values: `AllRecipients` (used when omitted), `MailboxUsers`, `MailContacts`, `MailGroups`, `MailUsers`, `Resources`."),
			"conditional_department":          optionalString("Filter by department attribute (comma-separated for multiple values)."),
			"conditional_company":             optionalString("Filter by company attribute (comma-separated for multiple values)."),
			"apply_to_recipients": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Run `Update-EmailAddressPolicy` after the policy is created, and after an apply that changes its settings, so that the existing recipients " +
					"matching its filter get the new addresses. Defaults to `false`: only the recipients created or modified afterwards are updated. " +
					"Changing only this attribute applies nothing. Applying a policy to many recipients takes time: raise `EXCHANGE_SCRIPT_TIMEOUT` if needed. " +
					"This is a setting of the provider, not of Exchange: an imported policy has none, so the first plan after an import shows it as a change, which only records it.",
			},
		},
	}
}

func (r *emailAddressPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *emailAddressPolicyResource) buildParams(ctx context.Context, action string, cfg emailAddressPolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	if known(cfg.EnabledEmailAddressTemplates) {
		p["EnabledEmailAddressTemplates"] = listToCSV(ctx, cfg.EnabledEmailAddressTemplates)
	}
	putInt64(p, "Priority", cfg.Priority)
	putString(p, "IncludedRecipients", cfg.IncludedRecipients)
	putString(p, "ConditionalDepartment", cfg.ConditionalDepartment)
	putString(p, "ConditionalCompany", cfg.ConditionalCompany)
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

// settingsChanged reports whether the plan changes a setting of the policy
// itself, apply_to_recipients aside: only then may an update apply the policy
// to the existing recipients.
func (m emailAddressPolicyModel) settingsChanged(state emailAddressPolicyModel) bool {
	return !m.EnabledEmailAddressTemplates.Equal(state.EnabledEmailAddressTemplates) ||
		!m.Priority.Equal(state.Priority) ||
		!m.IncludedRecipients.Equal(state.IncludedRecipients) ||
		!m.ConditionalDepartment.Equal(state.ConditionalDepartment) ||
		!m.ConditionalCompany.Equal(state.ConditionalCompany)
}

func (r *emailAddressPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan emailAddressPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg emailAddressPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params := r.buildParams(ctx, "Create", cfg, "")
	if cfg.ApplyToRecipients.ValueBool() {
		params["ApplyToRecipients"] = "true"
	}
	output, err := r.client.RunScript(ctx, "email_address_policy.ps1", params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create email address policy", err.Error())
		return
	}
	var a emailAddressPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	state := emailAddressPolicyFromAPI(a)
	state.ApplyToRecipients = plan.ApplyToRecipients
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *emailAddressPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state emailAddressPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "email_address_policy.ps1", map[string]string{"Action": "Get", "Identity": state.ID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read email address policy", err.Error())
		return
	}
	var a emailAddressPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	read := emailAddressPolicyFromAPI(a)
	read.ApplyToRecipients = state.ApplyToRecipients
	resp.Diagnostics.Append(resp.State.Set(ctx, read)...)
}

func (r *emailAddressPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state emailAddressPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg emailAddressPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params := r.buildParams(ctx, "Update", cfg, state.ID.ValueString())
	if cfg.ApplyToRecipients.ValueBool() && plan.settingsChanged(state) {
		params["ApplyToRecipients"] = "true"
	}
	output, err := r.client.RunScript(ctx, "email_address_policy.ps1", params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update email address policy", err.Error())
		return
	}
	var a emailAddressPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	updated := emailAddressPolicyFromAPI(a)
	updated.ApplyToRecipients = plan.ApplyToRecipients
	resp.Diagnostics.Append(resp.State.Set(ctx, updated)...)
}

func (r *emailAddressPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state emailAddressPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "email_address_policy.ps1", map[string]string{"Action": "Delete", "Identity": state.ID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete email address policy", err.Error())
	}
}

func (r *emailAddressPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func emailAddressPolicyFromAPI(a emailAddressPolicyAPIResponse) emailAddressPolicyModel {
	return emailAddressPolicyModel{
		ID:                           types.StringValue(a.Identity),
		Name:                         types.StringValue(a.Name),
		EnabledEmailAddressTemplates: stringsToList(a.EnabledEmailAddressTemplates),
		Priority:                     types.Int64Value(a.Priority),
		IncludedRecipients:           types.StringValue(a.IncludedRecipients),
		ConditionalDepartment:        types.StringValue(a.ConditionalDepartment),
		ConditionalCompany:           types.StringValue(a.ConditionalCompany),
		ApplyToRecipients:            types.BoolNull(),
	}
}
