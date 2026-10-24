package datasources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

var _ datasource.DataSource = &mailboxDataSource{}

type mailboxDataSource struct {
	client *client.Client
}

// mailboxDataModel is the read-only state returned to Terraform
type mailboxDataModel struct {
	ID                            types.String `tfsdk:"id"`
	Identity                      types.String `tfsdk:"identity"`
	DisplayName                   types.String `tfsdk:"display_name"`
	PrimarySmtpAddress            types.String `tfsdk:"primary_smtp_address"`
	RecipientTypeDetails          types.String `tfsdk:"recipient_type_details"`
	HiddenFromAddressListsEnabled types.Bool   `tfsdk:"hidden_from_address_lists_enabled"`
	LitigationHoldEnabled         types.Bool   `tfsdk:"litigation_hold_enabled"`
	RetentionPolicy               types.String `tfsdk:"retention_policy"`
	EmailAddresses                types.List   `tfsdk:"email_addresses"`
}

type mailboxAPIResponse struct {
	Identity                      string   `json:"Identity"`
	DisplayName                   string   `json:"DisplayName"`
	PrimarySmtpAddress            string   `json:"PrimarySmtpAddress"`
	RecipientTypeDetails          string   `json:"RecipientTypeDetails"`
	HiddenFromAddressListsEnabled bool     `json:"HiddenFromAddressListsEnabled"`
	LitigationHoldEnabled         bool     `json:"LitigationHoldEnabled"`
	RetentionPolicy               string   `json:"RetentionPolicy"`
	EmailAddresses                []string `json:"EmailAddresses"`
}

func NewMailboxDataSource() datasource.DataSource {
	return &mailboxDataSource{}
}

func (d *mailboxDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mailbox"
}

func (d *mailboxDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a mailbox. Mailboxes are read-only from this provider — use this data source to reference mailbox properties in other resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Exchange identity of the mailbox (same as `identity`).",
			},
			"identity": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The mailbox identity to look up. Accepts UPN, email address, alias, or display name.",
			},
			"display_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The display name of the mailbox.",
			},
			"primary_smtp_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The primary SMTP address of the mailbox.",
			},
			"recipient_type_details": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The mailbox type: `UserMailbox`, `SharedMailbox`, `RoomMailbox`, `EquipmentMailbox`, etc.",
			},
			"hidden_from_address_lists_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the mailbox is hidden from the Global Address List.",
			},
			"litigation_hold_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether litigation hold is enabled on the mailbox.",
			},
			"retention_policy": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The retention policy applied to the mailbox. Empty string if none.",
			},
			"email_addresses": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "All SMTP proxy addresses associated with the mailbox.",
			},
		},
	}
}

func (d *mailboxDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	apiClient, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = apiClient
}

func (d *mailboxDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config mailboxDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := d.client.RunScript(ctx, "mailbox.ps1", map[string]string{
		"Action":   "Get",
		"Identity": config.Identity.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read mailbox", err.Error())
		return
	}

	var apiResp mailboxAPIResponse
	if client.IsNotFound(output) {
		resp.Diagnostics.AddError("Mailbox not found", fmt.Sprintf("No mailbox found for identity %q", config.Identity.ValueString()))
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	emailList, diags := types.ListValueFrom(ctx, types.StringType, apiResp.EmailAddresses)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, mailboxDataModel{
		ID:                            types.StringValue(apiResp.Identity),
		Identity:                      types.StringValue(apiResp.Identity),
		DisplayName:                   types.StringValue(apiResp.DisplayName),
		PrimarySmtpAddress:            types.StringValue(apiResp.PrimarySmtpAddress),
		RecipientTypeDetails:          types.StringValue(apiResp.RecipientTypeDetails),
		HiddenFromAddressListsEnabled: types.BoolValue(apiResp.HiddenFromAddressListsEnabled),
		LitigationHoldEnabled:         types.BoolValue(apiResp.LitigationHoldEnabled),
		RetentionPolicy:               types.StringValue(apiResp.RetentionPolicy),
		EmailAddresses:                emailList,
	})...)
}
