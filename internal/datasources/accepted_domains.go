package datasources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

var _ datasource.DataSource = &acceptedDomainsDataSource{}

type acceptedDomainsDataSource struct {
	client *client.Client
}

// acceptedDomainsDataModel is the read-only state returned to Terraform
type acceptedDomainsDataModel struct {
	ID      types.String `tfsdk:"id"`
	Domains types.List   `tfsdk:"domains"`
}

// acceptedDomainItemAPIResponse maps one entry in the JSON array from PowerShell
type acceptedDomainItemAPIResponse struct {
	Identity        string `json:"Identity"`
	Name            string `json:"Name"`
	DomainName      string `json:"DomainName"`
	DomainType      string `json:"DomainType"`
	Default         bool   `json:"Default"`
	MatchSubDomains bool   `json:"MatchSubDomains"`
}

// domainObjectType defines the attribute types for the nested object in the schema
var domainObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"identity":          types.StringType,
		"name":              types.StringType,
		"domain_name":       types.StringType,
		"domain_type":       types.StringType,
		"default":           types.BoolType,
		"match_sub_domains": types.BoolType,
	},
}

func NewAcceptedDomainsDataSource() datasource.DataSource {
	return &acceptedDomainsDataSource{}
}

func (d *acceptedDomainsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_accepted_domains"
}

func (d *acceptedDomainsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every accepted domain of the organization, without managing them. Use it to filter or iterate over the domains, for example to scope a policy to all authoritative domains.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Always `accepted_domains`.",
			},
			"domains": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of accepted domains.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"identity": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the accepted domain.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the accepted domain entry.",
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
				},
			},
		},
	}
}

func (d *acceptedDomainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *acceptedDomainsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	output, err := d.client.RunScript(ctx, "accepted_domain.ps1", map[string]string{
		"Action": "List",
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to list accepted domains", err.Error())
		return
	}

	var apiItems []acceptedDomainItemAPIResponse
	if err := json.Unmarshal(output, &apiItems); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	domainValues := make([]attr.Value, len(apiItems))
	for i, item := range apiItems {
		obj, diags := types.ObjectValue(domainObjectType.AttrTypes, map[string]attr.Value{
			"identity":          types.StringValue(item.Identity),
			"name":              types.StringValue(item.Name),
			"domain_name":       types.StringValue(item.DomainName),
			"domain_type":       types.StringValue(item.DomainType),
			"default":           types.BoolValue(item.Default),
			"match_sub_domains": types.BoolValue(item.MatchSubDomains),
		})
		resp.Diagnostics.Append(diags...)
		domainValues[i] = obj
	}
	if resp.Diagnostics.HasError() {
		return
	}

	domainList, diags := types.ListValue(domainObjectType, domainValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, acceptedDomainsDataModel{
		ID:      types.StringValue("accepted_domains"),
		Domains: domainList,
	})...)
}
