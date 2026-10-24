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

var _ resource.Resource = &remoteDomainResource{}
var _ resource.ResourceWithImportState = &remoteDomainResource{}

type remoteDomainResource struct {
	client *client.Client
}

// remoteDomainModel represents the Terraform state of a remote domain
type remoteDomainModel struct {
	ID                         types.String `tfsdk:"id"`
	Name                       types.String `tfsdk:"name"`
	DomainName                 types.String `tfsdk:"domain_name"`
	AutoReplyEnabled           types.Bool   `tfsdk:"auto_reply_enabled"`
	AutoForwardEnabled         types.Bool   `tfsdk:"auto_forward_enabled"`
	TrustedMailInboundEnabled  types.Bool   `tfsdk:"trusted_mail_inbound_enabled"`
	TrustedMailOutboundEnabled types.Bool   `tfsdk:"trusted_mail_outbound_enabled"`
	// tnef_enabled is tri-state (true/false/inherit) — stored as "" when inheriting org default
	TnefEnabled    types.String `tfsdk:"tnef_enabled"`
	AllowedOofType types.String `tfsdk:"allowed_oof_type"`
}

// remoteDomainAPIResponse maps the JSON returned by the PowerShell bridge
type remoteDomainAPIResponse struct {
	Identity                   string `json:"Identity"`
	Name                       string `json:"Name"`
	DomainName                 string `json:"DomainName"`
	AutoReplyEnabled           bool   `json:"AutoReplyEnabled"`
	AutoForwardEnabled         bool   `json:"AutoForwardEnabled"`
	TrustedMailInboundEnabled  bool   `json:"TrustedMailInboundEnabled"`
	TrustedMailOutboundEnabled bool   `json:"TrustedMailOutboundEnabled"`
	TnefEnabled                *bool  `json:"TnefEnabled"` // pointer — null means "inherit org default"
	AllowedOofType             string `json:"AllowedOofType"`
}

func NewRemoteDomainResource() resource.Resource {
	return &remoteDomainResource{}
}

func (r *remoteDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_remote_domain"
}

func (r *remoteDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a remote domain (email routing and security settings for a specific external domain).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the remote domain (same as `name`).",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name for the remote domain entry.",
				// Exchange does not support renaming — must destroy and recreate
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"domain_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The SMTP domain (e.g. `partner-bank.com`). Immutable after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"auto_reply_enabled":            optionalBool("Allow automatic replies to recipients in this domain."),
			"auto_forward_enabled":          optionalBool("Allow auto-forwarding of messages to recipients in this domain."),
			"trusted_mail_inbound_enabled":  optionalBool("Trust inbound mail from this domain (bypass some spam filters)."),
			"trusted_mail_outbound_enabled": optionalBool("Trust outbound mail to this domain."),
			"tnef_enabled":                  optionalString("TNEF (Winmail.dat) encoding. `\"true\"` to force, `\"false\"` to disable, `\"\"` to inherit the organization default."),
			"allowed_oof_type":              optionalString("Type of out-of-office messages allowed. Valid values: `External`, `ExternalLegacy`, `InternalLegacy`, `None`."),
		},
	}
}

func (r *remoteDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *remoteDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan remoteDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg remoteDomainModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "remote_domain.ps1", r.buildParams("Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create remote domain", err.Error())
		return
	}

	var apiResp remoteDomainAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, remoteDomainFromAPI(apiResp))...)
}

func (r *remoteDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state remoteDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "remote_domain.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read remote domain", err.Error())
		return
	}

	var apiResp remoteDomainAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, remoteDomainFromAPI(apiResp))...)
}

func (r *remoteDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan remoteDomainModel
	var state remoteDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg remoteDomainModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "remote_domain.ps1", r.buildParams("Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update remote domain", err.Error())
		return
	}

	var apiResp remoteDomainAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, remoteDomainFromAPI(apiResp))...)
}

func (r *remoteDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state remoteDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "remote_domain.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete remote domain", err.Error())
	}
}

func (r *remoteDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildParams assembles the PowerShell parameter map for Create and Update
func (r *remoteDomainResource) buildParams(action string, cfg remoteDomainModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString(), "DomainName": cfg.DomainName.ValueString()}
	putBool(p, "AutoReplyEnabled", cfg.AutoReplyEnabled)
	putBool(p, "AutoForwardEnabled", cfg.AutoForwardEnabled)
	putBool(p, "TrustedMailInboundEnabled", cfg.TrustedMailInboundEnabled)
	putBool(p, "TrustedMailOutboundEnabled", cfg.TrustedMailOutboundEnabled)
	putString(p, "TnefEnabled", cfg.TnefEnabled)
	putString(p, "AllowedOofType", cfg.AllowedOofType)
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

// remoteDomainFromAPI converts the PowerShell JSON response to the Terraform state model.
// TnefEnabled is tri-state (true/false/null) — stored as "" when null (inherit org default).
func remoteDomainFromAPI(a remoteDomainAPIResponse) remoteDomainModel {
	tnef := ""
	if a.TnefEnabled != nil {
		tnef = fmt.Sprintf("%t", *a.TnefEnabled)
	}
	return remoteDomainModel{
		ID:                         types.StringValue(a.Identity),
		Name:                       types.StringValue(a.Name),
		DomainName:                 types.StringValue(a.DomainName),
		AutoReplyEnabled:           types.BoolValue(a.AutoReplyEnabled),
		AutoForwardEnabled:         types.BoolValue(a.AutoForwardEnabled),
		TrustedMailInboundEnabled:  types.BoolValue(a.TrustedMailInboundEnabled),
		TrustedMailOutboundEnabled: types.BoolValue(a.TrustedMailOutboundEnabled),
		TnefEnabled:                types.StringValue(tnef),
		AllowedOofType:             types.StringValue(a.AllowedOofType),
	}
}
