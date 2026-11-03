package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// The certificate data source shares exchange_certificate.ps1 and its
// response with the resources, so it lives next to them rather than in
// internal/datasources. It reads every certificate of a server, built-in
// ones included, whether or not Terraform manages it.

var _ datasource.DataSourceWithConfigure = &exchangeCertificateDataSource{}

type exchangeCertificateDataSource struct {
	client *client.Client
}

type exchangeCertificateDataModel struct {
	ID                   types.String `tfsdk:"id"`
	Server               types.String `tfsdk:"server"`
	Thumbprint           types.String `tfsdk:"thumbprint"`
	FriendlyName         types.String `tfsdk:"friendly_name"`
	Subject              types.String `tfsdk:"subject"`
	Issuer               types.String `tfsdk:"issuer"`
	CertificateDomains   types.List   `tfsdk:"certificate_domains"`
	NotBefore            types.String `tfsdk:"not_before"`
	NotAfter             types.String `tfsdk:"not_after"`
	Status               types.String `tfsdk:"status"`
	SelfSigned           types.Bool   `tfsdk:"self_signed"`
	HasPrivateKey        types.Bool   `tfsdk:"has_private_key"`
	PrivateKeyExportable types.Bool   `tfsdk:"private_key_exportable"`
	Services             types.List   `tfsdk:"services"`
}

func NewExchangeCertificateDataSource() datasource.DataSource {
	return &exchangeCertificateDataSource{}
}

func (d *exchangeCertificateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exchange_certificate"
}

func (d *exchangeCertificateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, MarkdownDescription: desc}
	}
	boolean := func(desc string) schema.BoolAttribute {
		return schema.BoolAttribute{Computed: true, MarkdownDescription: desc}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a certificate of an Exchange server (`Get-ExchangeCertificate`), such as the one a service uses " +
			"before it is replaced, or a certificate that this configuration does not manage.",
		Attributes: map[string]schema.Attribute{
			"id": str("The identity of the certificate, `<server>\\<thumbprint>`."),
			"server": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Exchange server that holds the certificate, as a name or an FQDN.",
			},
			"thumbprint": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The thumbprint of the certificate: 40 hexadecimal characters.",
			},
			"friendly_name": str("The friendly name of the certificate on the server."),
			"subject":       str("The subject of the certificate, for example `CN=mail.contoso.com`."),
			"issuer":        str("The issuer of the certificate."),
			"certificate_domains": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "The domains of the certificate: its subject and subject alternative names.",
			},
			"not_before":             str("The start of the validity period, in UTC (RFC 3339)."),
			"not_after":              str("The end of the validity period, in UTC (RFC 3339)."),
			"status":                 str("The status Exchange reports, for example `Valid`, `Invalid` or `RevocationCheckFailure`."),
			"self_signed":            boolean("Whether the certificate is self-signed."),
			"has_private_key":        boolean("Whether the server has the private key of the certificate."),
			"private_key_exportable": boolean("Whether the private key can be exported from the server."),
			"services": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				MarkdownDescription: "The services the certificate is enabled for: `IIS`, `IMAP`, `POP`, and `SMTP` when it is the default SMTP " +
					"certificate of the server. Empty when it has none.",
			},
		},
	}
}

func (d *exchangeCertificateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *exchangeCertificateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config exchangeCertificateDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := certificateID(config.Server.ValueString(), config.Thumbprint.ValueString())
	cert, err := runCertificateScript(ctx, d.client, map[string]string{"Action": "Get", "Identity": id})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the certificate", err.Error())
		return
	}
	if cert == nil {
		resp.Diagnostics.AddError("Certificate not found", fmt.Sprintf("No certificate %s exists on %s.", config.Thumbprint.ValueString(), config.Server.ValueString()))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, exchangeCertificateDataModel{
		ID:                   types.StringValue(id),
		Server:               config.Server,
		Thumbprint:           config.Thumbprint,
		FriendlyName:         types.StringValue(cert.FriendlyName),
		Subject:              types.StringValue(cert.Subject),
		Issuer:               types.StringValue(cert.Issuer),
		CertificateDomains:   stringsToList(cert.CertificateDomains),
		NotBefore:            types.StringValue(cert.NotBefore),
		NotAfter:             types.StringValue(cert.NotAfter),
		Status:               types.StringValue(cert.Status),
		SelfSigned:           types.BoolValue(cert.IsSelfSigned),
		HasPrivateKey:        types.BoolValue(cert.HasPrivateKey),
		PrivateKeyExportable: types.BoolValue(cert.PrivateKeyExportable),
		Services:             stringsToList(cert.Services),
	})...)
}
