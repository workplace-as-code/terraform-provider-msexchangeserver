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

var _ datasource.DataSource = &remoteMailboxDataSource{}

type remoteMailboxDataSource struct {
	client *client.Client
}

// remoteMailboxDataModel is the read-only state returned to Terraform
type remoteMailboxDataModel struct {
	ID                            types.String `tfsdk:"id"`
	Identity                      types.String `tfsdk:"identity"`
	DisplayName                   types.String `tfsdk:"display_name"`
	Alias                         types.String `tfsdk:"alias"`
	UserPrincipalName             types.String `tfsdk:"user_principal_name"`
	PrimarySmtpAddress            types.String `tfsdk:"primary_smtp_address"`
	RemoteRoutingAddress          types.String `tfsdk:"remote_routing_address"`
	RecipientTypeDetails          types.String `tfsdk:"recipient_type_details"`
	ExchangeGUID                  types.String `tfsdk:"exchange_guid"`
	HiddenFromAddressListsEnabled types.Bool   `tfsdk:"hidden_from_address_lists_enabled"`
	EmailAddressPolicyEnabled     types.Bool   `tfsdk:"email_address_policy_enabled"`
	EmailAddresses                types.List   `tfsdk:"email_addresses"`
}

type remoteMailboxAPIResponse struct {
	Identity                      string   `json:"Identity"`
	DisplayName                   string   `json:"DisplayName"`
	Alias                         string   `json:"Alias"`
	UserPrincipalName             string   `json:"UserPrincipalName"`
	PrimarySmtpAddress            string   `json:"PrimarySmtpAddress"`
	RemoteRoutingAddress          string   `json:"RemoteRoutingAddress"`
	RecipientTypeDetails          string   `json:"RecipientTypeDetails"`
	ExchangeGUID                  string   `json:"ExchangeGuid"`
	HiddenFromAddressListsEnabled bool     `json:"HiddenFromAddressListsEnabled"`
	EmailAddressPolicyEnabled     bool     `json:"EmailAddressPolicyEnabled"`
	EmailAddresses                []string `json:"EmailAddresses"`
}

func NewRemoteMailboxDataSource() datasource.DataSource {
	return &remoteMailboxDataSource{}
}

func (d *remoteMailboxDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_remote_mailbox"
}

func (d *remoteMailboxDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, MarkdownDescription: desc}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a remote mailbox (`Get-RemoteMailbox`): the on-premises object of a mailbox hosted in Exchange Online, " +
			"in a hybrid organization. Remote mailboxes are read-only from this provider.",
		Attributes: map[string]schema.Attribute{
			"id": str("The Exchange identity of the remote mailbox (same as `identity`)."),
			"identity": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The remote mailbox to look up: UPN, email address, alias or distinguished name.",
			},
			"display_name":           str("The display name."),
			"alias":                  str("The alias."),
			"user_principal_name":    str("The user principal name."),
			"primary_smtp_address":   str("The primary SMTP address."),
			"remote_routing_address": str("The address mail is routed to in Exchange Online, usually `<alias>@<tenant>.mail.onmicrosoft.com`, without the `SMTP:` prefix."),
			"recipient_type_details": str("The remote mailbox type: `RemoteUserMailbox`, `RemoteSharedMailbox`, `RemoteRoomMailbox` or `RemoteEquipmentMailbox`."),
			"exchange_guid":          str("The Exchange GUID, which must match the one of the mailbox in Exchange Online."),
			"hidden_from_address_lists_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the remote mailbox is hidden from the address lists.",
			},
			"email_address_policy_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether email address policies set the addresses of the remote mailbox.",
			},
			"email_addresses": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "All SMTP proxy addresses, without the `smtp:` prefix.",
			},
		},
	}
}

func (d *remoteMailboxDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *remoteMailboxDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config remoteMailboxDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := d.client.RunScript(ctx, "remote_mailbox.ps1", map[string]string{
		"Action":   "Get",
		"Identity": config.Identity.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read remote mailbox", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.Diagnostics.AddError("Remote mailbox not found", fmt.Sprintf("No remote mailbox found for identity %q", config.Identity.ValueString()))
		return
	}

	var apiResp remoteMailboxAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	emailList, diags := types.ListValueFrom(ctx, types.StringType, apiResp.EmailAddresses)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, remoteMailboxDataModel{
		ID:                            types.StringValue(apiResp.Identity),
		Identity:                      config.Identity,
		DisplayName:                   types.StringValue(apiResp.DisplayName),
		Alias:                         types.StringValue(apiResp.Alias),
		UserPrincipalName:             types.StringValue(apiResp.UserPrincipalName),
		PrimarySmtpAddress:            types.StringValue(apiResp.PrimarySmtpAddress),
		RemoteRoutingAddress:          types.StringValue(apiResp.RemoteRoutingAddress),
		RecipientTypeDetails:          types.StringValue(apiResp.RecipientTypeDetails),
		ExchangeGUID:                  types.StringValue(apiResp.ExchangeGUID),
		HiddenFromAddressListsEnabled: types.BoolValue(apiResp.HiddenFromAddressListsEnabled),
		EmailAddressPolicyEnabled:     types.BoolValue(apiResp.EmailAddressPolicyEnabled),
		EmailAddresses:                emailList,
	})...)
}
