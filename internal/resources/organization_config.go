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

var _ resource.Resource = &organizationConfigResource{}
var _ resource.ResourceWithImportState = &organizationConfigResource{}

// organizationConfigResource is a singleton — there is exactly one org config per Exchange organization.
// Create and Delete don't call Exchange (you can't create or destroy the org config object),
// they just manage Terraform state. Only Read and Update make real API calls.
type organizationConfigResource struct {
	client *client.Client
}

type organizationConfigModel struct {
	// ID is always "organization" — it's a singleton with no real identifier
	ID                                    types.String `tfsdk:"id"`
	ReadTrackingEnabled                   types.Bool   `tfsdk:"read_tracking_enabled"`
	MailTipsAllTipsEnabled                types.Bool   `tfsdk:"mail_tips_all_tips_enabled"`
	MailTipsExternalRecipientsTipsEnabled types.Bool   `tfsdk:"mail_tips_external_recipients_tips_enabled"`
	MailTipsLargeAudienceThreshold        types.Int64  `tfsdk:"mail_tips_large_audience_threshold"`
	OAuth2ClientProfileEnabled            types.Bool   `tfsdk:"oauth2_client_profile_enabled"`
}

type organizationConfigAPIResponse struct {
	Identity                              string `json:"Identity"`
	ReadTrackingEnabled                   bool   `json:"ReadTrackingEnabled"`
	MailTipsAllTipsEnabled                bool   `json:"MailTipsAllTipsEnabled"`
	MailTipsExternalRecipientsTipsEnabled bool   `json:"MailTipsExternalRecipientsTipsEnabled"`
	MailTipsLargeAudienceThreshold        int64  `json:"MailTipsLargeAudienceThreshold"`
	OAuth2ClientProfileEnabled            bool   `json:"OAuth2ClientProfileEnabled"`
}

func NewOrganizationConfigResource() resource.Resource {
	return &organizationConfigResource{}
}

func (r *organizationConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_config"
}

func (r *organizationConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages Exchange organization-wide configuration settings (`Set-OrganizationConfig`). This is a singleton resource: there is exactly one organization config per organization. Only the attributes set in the configuration are managed; the others keep their current value. Destroying this resource removes it from Terraform state without changing the Exchange configuration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Always `\"organization\"`. This is a singleton resource.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"read_tracking_enabled":                      optionalBool("Enable read tracking so senders can see when their messages were read."),
			"mail_tips_all_tips_enabled":                 optionalBool("Enable MailTips for the organization. When `false`, all MailTips are disabled regardless of individual settings."),
			"mail_tips_external_recipients_tips_enabled": optionalBool("Show a MailTip when composing a message to external recipients."),
			"mail_tips_large_audience_threshold":         optionalInt64("Number of recipients above which the large-audience MailTip is shown."),
			"oauth2_client_profile_enabled":              optionalBool("Enable modern authentication (OAuth 2.0) for Outlook and other clients (hybrid modern authentication)."),
		},
	}
}

func (r *organizationConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create applies the desired settings (Set-OrganizationConfig) and reads back the result.
// There is no "New-OrganizationConfig" — the object always exists.
func (r *organizationConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg organizationConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "organization_config.ps1", r.buildSetParams(cfg))
	if err != nil {
		resp.Diagnostics.AddError("Failed to configure organization settings", err.Error())
		return
	}

	var apiResp organizationConfigAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, organizationConfigFromAPI(apiResp))...)
}

func (r *organizationConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	output, err := r.client.RunScript(ctx, "organization_config.ps1", map[string]string{
		"Action": "Get",
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read organization config", err.Error())
		return
	}

	var apiResp organizationConfigAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, organizationConfigFromAPI(apiResp))...)
}

func (r *organizationConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan organizationConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg organizationConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "organization_config.ps1", r.buildSetParams(cfg))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update organization config", err.Error())
		return
	}

	var apiResp organizationConfigAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, organizationConfigFromAPI(apiResp))...)
}

// Delete removes the resource from Terraform state without touching Exchange.
// Organization config cannot be deleted — the object always exists in the organization.
func (r *organizationConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	warnStateOnlyDestroy(&resp.Diagnostics, "organization_config", "The organization configuration cannot be deleted.")
}

func (r *organizationConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildSetParams sends only the attributes known in the plan: an attribute
// left out of the configuration keeps its current Exchange value.
func (r *organizationConfigResource) buildSetParams(plan organizationConfigModel) map[string]string {
	p := map[string]string{"Action": "Set"}
	putBool(p, "ReadTrackingEnabled", plan.ReadTrackingEnabled)
	putBool(p, "MailTipsAllTipsEnabled", plan.MailTipsAllTipsEnabled)
	putBool(p, "MailTipsExternalRecipientsTipsEnabled", plan.MailTipsExternalRecipientsTipsEnabled)
	putInt64(p, "MailTipsLargeAudienceThreshold", plan.MailTipsLargeAudienceThreshold)
	putBool(p, "OAuth2ClientProfileEnabled", plan.OAuth2ClientProfileEnabled)
	return p
}

func organizationConfigFromAPI(a organizationConfigAPIResponse) organizationConfigModel {
	return organizationConfigModel{
		ID:                                    types.StringValue("organization"),
		ReadTrackingEnabled:                   types.BoolValue(a.ReadTrackingEnabled),
		MailTipsAllTipsEnabled:                types.BoolValue(a.MailTipsAllTipsEnabled),
		MailTipsExternalRecipientsTipsEnabled: types.BoolValue(a.MailTipsExternalRecipientsTipsEnabled),
		MailTipsLargeAudienceThreshold:        types.Int64Value(a.MailTipsLargeAudienceThreshold),
		OAuth2ClientProfileEnabled:            types.BoolValue(a.OAuth2ClientProfileEnabled),
	}
}
