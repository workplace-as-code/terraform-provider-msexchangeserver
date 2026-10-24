package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

var _ resource.Resource = &owaMailboxPolicyResource{}
var _ resource.ResourceWithImportState = &owaMailboxPolicyResource{}

type owaMailboxPolicyResource struct {
	client *client.Client
}

// owaMailboxPolicyModel represents the Terraform state of an OWA mailbox policy
type owaMailboxPolicyModel struct {
	ID                                  types.String `tfsdk:"id"`
	Name                                types.String `tfsdk:"name"`
	InstantMessagingEnabled             types.Bool   `tfsdk:"instant_messaging_enabled"`
	ActiveSyncIntegrationEnabled        types.Bool   `tfsdk:"active_sync_integration_enabled"`
	AttachmentsEnabled                  types.Bool   `tfsdk:"attachments_enabled"`
	PublicFoldersEnabled                types.Bool   `tfsdk:"public_folders_enabled"`
	RecoverDeletedItemsEnabled          types.Bool   `tfsdk:"recover_deleted_items_enabled"`
	RulesEnabled                        types.Bool   `tfsdk:"rules_enabled"`
	SignaturesEnabled                   types.Bool   `tfsdk:"signatures_enabled"`
	ThemeSelectionEnabled               types.Bool   `tfsdk:"theme_selection_enabled"`
	WacViewingOnPrivateComputersEnabled types.Bool   `tfsdk:"wac_viewing_on_private_computers_enabled"`
	WacViewingOnPublicComputersEnabled  types.Bool   `tfsdk:"wac_viewing_on_public_computers_enabled"`
	WebPartsFrameOptionsType            types.String `tfsdk:"web_parts_frame_options_type"`
	OutboundCharset                     types.String `tfsdk:"outbound_charset"`
	BlockedFileTypes                    types.Set    `tfsdk:"blocked_file_types"`
	BlockedMimeTypes                    types.Set    `tfsdk:"blocked_mime_types"`
}

// owaMailboxPolicyAPIResponse maps the JSON returned by the PowerShell bridge
type owaMailboxPolicyAPIResponse struct {
	Identity                            string          `json:"Identity"`
	Name                                string          `json:"Name"`
	InstantMessagingEnabled             bool            `json:"InstantMessagingEnabled"`
	ActiveSyncIntegrationEnabled        bool            `json:"ActiveSyncIntegrationEnabled"`
	AttachmentsEnabled                  bool            `json:"AttachmentsEnabled"`
	PublicFoldersEnabled                bool            `json:"PublicFoldersEnabled"`
	RecoverDeletedItemsEnabled          bool            `json:"RecoverDeletedItemsEnabled"`
	RulesEnabled                        bool            `json:"RulesEnabled"`
	SignaturesEnabled                   bool            `json:"SignaturesEnabled"`
	ThemeSelectionEnabled               bool            `json:"ThemeSelectionEnabled"`
	WacViewingOnPrivateComputersEnabled bool            `json:"WacViewingOnPrivateComputersEnabled"`
	WacViewingOnPublicComputersEnabled  bool            `json:"WacViewingOnPublicComputersEnabled"`
	WebPartsFrameOptionsType            string          `json:"WebPartsFrameOptionsType"`
	OutboundCharset                     string          `json:"OutboundCharset"`
	BlockedFileTypes                    FlexStringSlice `json:"BlockedFileTypes"`
	BlockedMimeTypes                    FlexStringSlice `json:"BlockedMimeTypes"`
}

func NewOwaMailboxPolicyResource() resource.Resource {
	return &owaMailboxPolicyResource{}
}

func (r *owaMailboxPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_owa_mailbox_policy"
}

func (r *owaMailboxPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an OWA (Outlook on the web) mailbox policy. Policies are assigned to mailboxes via `Set-CASMailbox -OwaMailboxPolicy`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the OWA mailbox policy.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the OWA mailbox policy. Immutable after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"instant_messaging_enabled":                optionalBool("Allow instant messaging in Outlook on the Web."),
			"active_sync_integration_enabled":          optionalBool("Show ActiveSync device management options in Outlook on the Web."),
			"attachments_enabled":                      optionalBool("Allow users to open and send attachments in Outlook on the Web."),
			"public_folders_enabled":                   optionalBool("Allow access to public folders in Outlook on the Web."),
			"recover_deleted_items_enabled":            optionalBool("Allow users to recover deleted items from the Recoverable Items folder."),
			"rules_enabled":                            optionalBool("Allow users to create and manage inbox rules in Outlook on the Web."),
			"signatures_enabled":                       optionalBool("Allow users to create and use email signatures."),
			"theme_selection_enabled":                  optionalBool("Allow users to select a theme in Outlook on the Web."),
			"wac_viewing_on_private_computers_enabled": optionalBool("Allow Office Online (WAC) document viewing on private computers."),
			"wac_viewing_on_public_computers_enabled":  optionalBool("Allow Office Online (WAC) document viewing on public computers."),
			"web_parts_frame_options_type":             optionalString("X-Frame-Options header behavior for Outlook on the Web. Valid values: `None`, `SameOrigin`, `Deny`."),
			"outbound_charset":                         optionalString("Character set for outbound messages. Valid values: `AlwaysUTF8`, `AutoDetect`, `UserLanguageChoice`."),
			"blocked_file_types": schema.SetAttribute{
				PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "File extensions blocked as email attachments in Outlook on the Web (e.g. `[\"exe\", \"bat\"]`). Order does not matter.",
			},
			"blocked_mime_types": schema.SetAttribute{
				PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "MIME types blocked as email attachments in Outlook on the Web (e.g. `[\"application/x-javascript\"]`). Order does not matter.",
			},
		},
	}
}

func (r *owaMailboxPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *owaMailboxPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan owaMailboxPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg owaMailboxPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "owa_mailbox_policy.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create OWA mailbox policy", err.Error())
		return
	}

	var apiResp owaMailboxPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, owaMailboxPolicyFromAPI(apiResp))...)
}

func (r *owaMailboxPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state owaMailboxPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "owa_mailbox_policy.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read OWA mailbox policy", err.Error())
		return
	}

	var apiResp owaMailboxPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, owaMailboxPolicyFromAPI(apiResp))...)
}

func (r *owaMailboxPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan owaMailboxPolicyModel
	var state owaMailboxPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg owaMailboxPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "owa_mailbox_policy.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update OWA mailbox policy", err.Error())
		return
	}

	var apiResp owaMailboxPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, owaMailboxPolicyFromAPI(apiResp))...)
}

func (r *owaMailboxPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state owaMailboxPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "owa_mailbox_policy.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete OWA mailbox policy", err.Error())
	}
}

func (r *owaMailboxPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildParams assembles the PowerShell parameter map for Create and Update
func (r *owaMailboxPolicyResource) buildParams(ctx context.Context, action string, cfg owaMailboxPolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putBool(p, "InstantMessagingEnabled", cfg.InstantMessagingEnabled)
	putBool(p, "ActiveSyncIntegrationEnabled", cfg.ActiveSyncIntegrationEnabled)
	putBool(p, "ClassicAttachmentsEnabled", cfg.AttachmentsEnabled)
	putBool(p, "PublicFoldersEnabled", cfg.PublicFoldersEnabled)
	putBool(p, "RecoverDeletedItemsEnabled", cfg.RecoverDeletedItemsEnabled)
	putBool(p, "RulesEnabled", cfg.RulesEnabled)
	putBool(p, "SignaturesEnabled", cfg.SignaturesEnabled)
	putBool(p, "ThemeSelectionEnabled", cfg.ThemeSelectionEnabled)
	putBool(p, "WacViewingOnPrivateComputersEnabled", cfg.WacViewingOnPrivateComputersEnabled)
	putBool(p, "WacViewingOnPublicComputersEnabled", cfg.WacViewingOnPublicComputersEnabled)
	putString(p, "WebPartsFrameOptionsType", cfg.WebPartsFrameOptionsType)
	putString(p, "OutboundCharset", cfg.OutboundCharset)
	if known(cfg.BlockedFileTypes) {
		p["BlockedFileTypes"] = setToCSV(ctx, cfg.BlockedFileTypes)
	}
	if known(cfg.BlockedMimeTypes) {
		p["BlockedMimeTypes"] = setToCSV(ctx, cfg.BlockedMimeTypes)
	}
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

// owaMailboxPolicyFromAPI converts the PowerShell JSON response to the Terraform state model
func owaMailboxPolicyFromAPI(a owaMailboxPolicyAPIResponse) owaMailboxPolicyModel {
	return owaMailboxPolicyModel{
		ID:                                  types.StringValue(a.Identity),
		Name:                                types.StringValue(a.Name),
		InstantMessagingEnabled:             types.BoolValue(a.InstantMessagingEnabled),
		ActiveSyncIntegrationEnabled:        types.BoolValue(a.ActiveSyncIntegrationEnabled),
		AttachmentsEnabled:                  types.BoolValue(a.AttachmentsEnabled),
		PublicFoldersEnabled:                types.BoolValue(a.PublicFoldersEnabled),
		RecoverDeletedItemsEnabled:          types.BoolValue(a.RecoverDeletedItemsEnabled),
		RulesEnabled:                        types.BoolValue(a.RulesEnabled),
		SignaturesEnabled:                   types.BoolValue(a.SignaturesEnabled),
		ThemeSelectionEnabled:               types.BoolValue(a.ThemeSelectionEnabled),
		WacViewingOnPrivateComputersEnabled: types.BoolValue(a.WacViewingOnPrivateComputersEnabled),
		WacViewingOnPublicComputersEnabled:  types.BoolValue(a.WacViewingOnPublicComputersEnabled),
		WebPartsFrameOptionsType:            types.StringValue(a.WebPartsFrameOptionsType),
		OutboundCharset:                     types.StringValue(a.OutboundCharset),
		BlockedFileTypes:                    stringsToSet(a.BlockedFileTypes),
		BlockedMimeTypes:                    stringsToSet(a.BlockedMimeTypes),
	}
}
