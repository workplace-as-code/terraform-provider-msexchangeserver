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

var _ datasource.DataSource = &organizationConfigDataSource{}

type organizationConfigDataSource struct {
	client *client.Client
}

// organizationConfigDataModel is the read-only state returned to Terraform
type organizationConfigDataModel struct {
	ID                  types.String `tfsdk:"id"`
	DisplayName         types.String `tfsdk:"display_name"`
	SCLJunkThreshold    types.Int64  `tfsdk:"scl_junk_threshold"`
	ReadTrackingEnabled types.Bool   `tfsdk:"read_tracking_enabled"`
}

type organizationConfigAPIResponse struct {
	Identity            string `json:"Identity"`
	DisplayName         string `json:"DisplayName"`
	SCLJunkThreshold    int64  `json:"SCLJunkThreshold"`
	ReadTrackingEnabled bool   `json:"ReadTrackingEnabled"`
}

func NewOrganizationConfigDataSource() datasource.DataSource {
	return &organizationConfigDataSource{}
}

func (d *organizationConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_config"
}

func (d *organizationConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the organization configuration. Use this data source to reference organization-wide settings in other resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Exchange identity of the organization configuration object.",
			},
			"display_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The display name of the organization.",
			},
			"scl_junk_threshold": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The spam confidence level (SCL) threshold above which messages are moved to the Junk Email folder.",
			},
			"read_tracking_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether read receipt tracking is enabled for the organization.",
			},
		},
	}
}

func (d *organizationConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *organizationConfigDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	output, err := d.client.RunScript(ctx, "organization_config.ps1", map[string]string{
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

	resp.Diagnostics.Append(resp.State.Set(ctx, organizationConfigDataModel{
		ID:                  types.StringValue(apiResp.Identity),
		DisplayName:         types.StringValue(apiResp.DisplayName),
		SCLJunkThreshold:    types.Int64Value(apiResp.SCLJunkThreshold),
		ReadTrackingEnabled: types.BoolValue(apiResp.ReadTrackingEnabled),
	})...)
}
