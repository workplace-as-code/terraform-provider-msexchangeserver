package provider

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/datasources"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/resources"
)

var _ provider.ProviderWithListResources = &exchangeProvider{}

type exchangeProvider struct {
	version string
}

// exchangeProviderModel represents the HCL provider {} block. Every field is
// optional: it can also come from an EXCHANGE_* environment variable.
type exchangeProviderModel struct {
	Server   types.String `tfsdk:"server"`
	Servers  types.List   `tfsdk:"servers"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Role     types.String `tfsdk:"role"`

	AuthMethod types.String `tfsdk:"auth_method"`

	DomainController types.String `tfsdk:"domain_controller"`
}

// New returns a factory function that creates the provider.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &exchangeProvider{version: version}
	}
}

func (p *exchangeProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "msexchangeserver"
	resp.Version = p.version
}

func (p *exchangeProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages Exchange Server configuration as code: the organization through a Mailbox server (Exchange remote PowerShell with Kerberos), or the settings of an Edge Transport server.",
		Attributes: map[string]schema.Attribute{
			"role": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Role of the server the provider connects to: `Mailbox` (default) manages the organization; `Edge` manages an Edge Transport server, " +
					"whose resources are limited to its own settings. Use one provider block per Edge server, with an alias. Can also be set via the `EXCHANGE_ROLE` environment variable.",
				Validators: []validator.String{stringvalidator.OneOfCaseInsensitive(client.RoleMailbox, client.RoleEdge)},
			},
			"server": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "FQDN of the Exchange server. With the `Mailbox` role, the FQDN registered in Active Directory (e.g. `exch01.contoso.local`): the connection uses Kerberos, which fails with an IP address " +
					"or a short name; a DNS alias or load balancer name needs an alternate service account (ASA), see the Authentication Guide. Required, either here or via the `EXCHANGE_SERVER` environment variable, unless `servers` is set. With the `Edge` role, the name WinRM reaches the Edge server with; " +
					"leave it out when Terraform runs on the Edge server itself.",
			},
			"servers": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				MarkdownDescription: "FQDNs of Exchange servers, as registered in Active Directory, tried in order: each connection goes to the first server that accepts it, and servers whose " +
					"`/PowerShell/healthcheck.htm` does not answer 200 (down or in maintenance) are tried last. Use it instead of `server`. An authentication failure is not retried on the next server. Exchange chooses the server that runs the commands, " +
					"whatever server the provider connects to: the list does not cover the failure of that server. `Mailbox` role only. " +
					"Can also be set via the `EXCHANGE_SERVERS` environment variable, comma-separated.",
				Validators: []validator.List{listvalidator.SizeAtLeast(1)},
			},
			"auth_method": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "How the provider authenticates with the `Mailbox` role. `password` (default): `username` and `password`. " +
					"`integrated`: Kerberos with the Windows identity of the process running Terraform, for example a CI runner service or a scheduled task " +
					"on a domain-joined machine; `username` and `password` must then be left unset. Not available with the `Edge` role. " +
					"Can also be set via the `EXCHANGE_AUTH_METHOD` environment variable.",
				Validators: []validator.String{stringvalidator.OneOfCaseInsensitive(client.AuthPassword, client.AuthIntegrated)},
			},
			"username": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "With the `Mailbox` role and `auth_method = \"password\"` (default), the Active Directory account used for Exchange Server, as `CONTOSO\\svc-terraform` or `svc-terraform@contoso.local`. With the `Edge` role and a `server`, " +
					"a local administrator of the Edge server, as `EDGE01\\Administrator`. Required in both cases, either here or via the `EXCHANGE_USERNAME` environment variable.",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Password of `username`. Not used with `auth_method = \"integrated\"`. Can also be set via the `EXCHANGE_PASSWORD` environment variable.",
			},
			"domain_controller": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "FQDN of the domain controller every session reads from and writes to (`Set-ADServerSettings -PreferredServer`), " +
					"so that two runs served by different Exchange servers see the same data. If it is unavailable, the connection fails. " +
					"Leave it out to let Exchange choose. `Mailbox` role only. Can also be set via the `EXCHANGE_DOMAIN_CONTROLLER` environment variable.",
			},
		},
	}
}

func (p *exchangeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config exchangeProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := resolveConfig(config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiClient, err := client.New(ctx, cfg)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Exchange client", err.Error())
		return
	}

	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
	resp.ListResourceData = apiClient
}

// Resources returns the list of all resources managed by the provider.
func (p *exchangeProvider) Resources(_ context.Context) []func() resource.Resource {
	return resources.Resources()
}

// ListResources lists the existing objects for terraform query (Terraform 1.14+).
func (p *exchangeProvider) ListResources(_ context.Context) []func() list.ListResource {
	return resources.ListResources()
}

// DataSources returns the list of all data sources.
func (p *exchangeProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewAcceptedDomainDataSource,
		datasources.NewAcceptedDomainsDataSource,
		datasources.NewMailboxDataSource,
		datasources.NewOrganizationConfigDataSource,
	}
}

// envOrConfig returns the HCL config value if set, otherwise the environment
// variable. An explicit provider block always wins, as in the HashiCorp
// providers, so a stray variable in a CI runner cannot silently redirect
// Terraform to another organization.
func envOrConfig(envKey string, configVal types.String) string {
	if !configVal.IsNull() && !configVal.IsUnknown() && configVal.ValueString() != "" {
		return configVal.ValueString()
	}
	return os.Getenv(envKey)
}

// normalizeRole returns the canonical name of a role given in any case, the
// Mailbox role when it is empty, and "" when it is not a role.
func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", "mailbox":
		return client.RoleMailbox
	case "edge":
		return client.RoleEdge
	}
	return ""
}
