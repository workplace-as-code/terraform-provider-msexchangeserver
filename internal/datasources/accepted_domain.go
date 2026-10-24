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

var _ datasource.DataSource = &acceptedDomainDataSource{}

type acceptedDomainDataSource struct {
	client *client.Client
}

// acceptedDomainDataModel is the read-only state returned to Terraform
type acceptedDomainDataModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	DomainName      types.String `tfsdk:"domain_name"`
	DomainType      types.String `tfsdk:"domain_type"`
	Default         types.Bool   `tfsdk:"default"`
	MatchSubDomains types.Bool   `tfsdk:"match_sub_domains"`
}

type acceptedDomainAPIResponse struct {
	Identity        string `json:"Identity"`
	Name            string `json:"Name"`
	DomainName      string `json:"DomainName"`
	DomainType      string `json:"DomainType"`
	Default         bool   `json:"Default"`
	MatchSubDomains bool   `json:"MatchSubDomains"`
}

func NewAcceptedDomainDataSource() datasource.DataSource {
	return &acceptedDomainDataSource{}
}

func (d *acceptedDomainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_accepted_domain"
}

func (d *acceptedDomainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one accepted domain by name, without managing it, and fails when it does not exist. Use it to reference a domain managed elsewhere; the `msexchangeserver_accepted_domain` resource manages its settings. To read every domain, use `msexchangeserver_accepted_domains`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the accepted domain.",
			},
			// name is the lookup key — required to identify which domain to read
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the accepted domain entry to look up.",
			},
			"domain_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The SMTP domain (e.g. `contoso.com`).",
			},
			"domain_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "How Exchange handles mail for this domain: `Authoritative`, `InternalRelay`, or `ExternalRelay` (Exchange Server only).",
			},
			"default": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this is the default accepted domain for the organization.",
			},
			"match_sub_domains": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether settings apply to all subdomains.",
			},
		},
	}
}

func (d *acceptedDomainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *acceptedDomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config acceptedDomainDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := d.client.RunScript(ctx, "accepted_domain.ps1", map[string]string{
		"Action":   "Get",
		"Identity": config.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read accepted domain", err.Error())
		return
	}

	var apiResp acceptedDomainAPIResponse
	if client.IsNotFound(output) {
		resp.Diagnostics.AddError("Accepted domain not found", fmt.Sprintf("No accepted domain with name %q", config.Name.ValueString()))
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, acceptedDomainDataModel{
		ID:              types.StringValue(apiResp.Identity),
		Name:            types.StringValue(apiResp.Name),
		DomainName:      types.StringValue(apiResp.DomainName),
		DomainType:      types.StringValue(apiResp.DomainType),
		Default:         types.BoolValue(apiResp.Default),
		MatchSubDomains: types.BoolValue(apiResp.MatchSubDomains),
	})...)
}
