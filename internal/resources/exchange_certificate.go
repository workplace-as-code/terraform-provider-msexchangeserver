package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// Exchange certificates: two resources and a data source, all through
// exchange_certificate.ps1.
//
//   - msexchangeserver_exchange_certificate imports a PFX on a server and
//     removes the certificate on destroy. The PFX and its password are
//     write-only attributes (Terraform 1.11 and later): they never reach the
//     plan or the state.
//   - msexchangeserver_exchange_certificate_services enables services on a
//     certificate (exchange_certificate_services.go).
//   - the data source reads any certificate of a server
//     (exchange_certificate_data_source.go).
//
// A certificate is identified by "<server>\<thumbprint>", as Exchange does.

var (
	_ resource.Resource                = &exchangeCertificateResource{}
	_ resource.ResourceWithImportState = &exchangeCertificateResource{}
)

// thumbprintPattern is the thumbprint as Exchange returns it: a value in lower
// case would show a difference on every plan.
var thumbprintPattern = regexp.MustCompile(`^[0-9A-F]{40}$`)

func thumbprintAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.RegexMatches(thumbprintPattern, "must be 40 hexadecimal characters in upper case"),
		},
		PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		MarkdownDescription: "The thumbprint of the certificate: 40 hexadecimal characters in upper case.",
	}
}

func certificateServerAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required:            true,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		MarkdownDescription: "The Exchange server that holds the certificate, as a name or an FQDN. Each server has its own certificates.",
	}
}

// certificateID is the identity of a certificate: "<server>\<thumbprint>".
func certificateID(server, thumbprint string) string {
	return server + `\` + thumbprint
}

// parseCertificateID splits an import ID into the server and the thumbprint.
func parseCertificateID(id string) (server, thumbprint string, err error) {
	server, thumbprint, ok := strings.Cut(id, `\`)
	if !ok || server == "" || !thumbprintPattern.MatchString(thumbprint) {
		return "", "", fmt.Errorf(`expected "<server>\<thumbprint>" with a thumbprint of 40 hexadecimal characters in upper case, got %q`, id)
	}
	return server, thumbprint, nil
}

// certificateAPIResponse is the certificate as exchange_certificate.ps1 returns it.
type certificateAPIResponse struct {
	Identity             string          `json:"Identity"`
	Thumbprint           string          `json:"Thumbprint"`
	FriendlyName         string          `json:"FriendlyName"`
	Subject              string          `json:"Subject"`
	Issuer               string          `json:"Issuer"`
	CertificateDomains   FlexStringSlice `json:"CertificateDomains"`
	NotBefore            string          `json:"NotBefore"`
	NotAfter             string          `json:"NotAfter"`
	Status               string          `json:"Status"`
	IsSelfSigned         bool            `json:"IsSelfSigned"`
	HasPrivateKey        bool            `json:"HasPrivateKey"`
	PrivateKeyExportable bool            `json:"PrivateKeyExportable"`
	Services             FlexStringSlice `json:"Services"`
}

// runCertificateScript runs exchange_certificate.ps1 and parses the
// certificate it returns; nil means the certificate does not exist.
func runCertificateScript(ctx context.Context, c *client.Client, params map[string]string) (*certificateAPIResponse, error) {
	output, err := c.RunScript(ctx, "exchange_certificate.ps1", params)
	if err != nil {
		return nil, err
	}
	if client.IsNotFound(output) {
		return nil, nil
	}
	var cert certificateAPIResponse
	if err := json.Unmarshal(output, &cert); err != nil {
		return nil, fmt.Errorf("failed to parse PowerShell response: %w", err)
	}
	return &cert, nil
}

type exchangeCertificateResource struct {
	client *client.Client
}

type exchangeCertificateModel struct {
	ID                   types.String `tfsdk:"id"`
	Server               types.String `tfsdk:"server"`
	Thumbprint           types.String `tfsdk:"thumbprint"`
	PfxBase64            types.String `tfsdk:"pfx_base64_wo"`
	PfxPassword          types.String `tfsdk:"pfx_password_wo"`
	FriendlyName         types.String `tfsdk:"friendly_name"`
	PrivateKeyExportable types.Bool   `tfsdk:"private_key_exportable"`
	Subject              types.String `tfsdk:"subject"`
	Issuer               types.String `tfsdk:"issuer"`
	CertificateDomains   types.List   `tfsdk:"certificate_domains"`
	NotBefore            types.String `tfsdk:"not_before"`
	NotAfter             types.String `tfsdk:"not_after"`
	SelfSigned           types.Bool   `tfsdk:"self_signed"`
}

// apply copies the certificate read from Exchange into the model. The
// write-only attributes are always null in the state.
func (m *exchangeCertificateModel) apply(a certificateAPIResponse) {
	m.ID = types.StringValue(certificateID(m.Server.ValueString(), m.Thumbprint.ValueString()))
	m.PfxBase64 = types.StringNull()
	m.PfxPassword = types.StringNull()
	m.FriendlyName = types.StringValue(a.FriendlyName)
	m.PrivateKeyExportable = types.BoolValue(a.PrivateKeyExportable)
	m.Subject = types.StringValue(a.Subject)
	m.Issuer = types.StringValue(a.Issuer)
	m.CertificateDomains = stringsToList(a.CertificateDomains)
	m.NotBefore = types.StringValue(a.NotBefore)
	m.NotAfter = types.StringValue(a.NotAfter)
	m.SelfSigned = types.BoolValue(a.IsSelfSigned)
}

func NewExchangeCertificateResource() resource.Resource {
	return &exchangeCertificateResource{}
}

func (r *exchangeCertificateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exchange_certificate"
}

func (r *exchangeCertificateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Computed:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			MarkdownDescription: desc,
		}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Imports a certificate with its private key (a PFX file) on an Exchange server, and removes it on destroy. " +
			"The PFX and its password are write-only attributes: they are sent to Exchange when the certificate is created and are never " +
			"stored in the plan or the state. Requires Terraform 1.11 or later. " +
			"Enable the certificate for services with `msexchangeserver_exchange_certificate_services`.",
		Attributes: map[string]schema.Attribute{
			"id":         computedString("The identity of the certificate, `<server>\\<thumbprint>`."),
			"server":     certificateServerAttribute(),
			"thumbprint": thumbprintAttribute(),
			"pfx_base64_wo": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				MarkdownDescription: "The PFX file, encoded in base64 (for example `filebase64(\"mail.pfx\")`, or an ephemeral value from a secret store). " +
					"Required to create the certificate: the provider checks that the PFX holds the certificate of `thumbprint` before importing it. " +
					"Write-only: changing it does not replace the certificate; change `thumbprint` to import a new one.",
			},
			"pfx_password_wo": schema.StringAttribute{
				Optional:            true,
				WriteOnly:           true,
				Sensitive:           true,
				MarkdownDescription: "The password of the PFX file. Write-only.",
			},
			"friendly_name": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "The friendly name of the certificate on the server. Changing it replaces the certificate.",
			},
			"private_key_exportable": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					boolplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "Whether the private key can be exported from the server after the import. Exchange's default is `false`. Changing it replaces the certificate.",
			},
			"subject": computedString("The subject of the certificate, for example `CN=mail.contoso.com`."),
			"issuer":  computedString("The issuer of the certificate."),
			"certificate_domains": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "The domains of the certificate: its subject and subject alternative names.",
			},
			"not_before": computedString("The start of the validity period, in UTC (RFC 3339)."),
			"not_after":  computedString("The end of the validity period, in UTC (RFC 3339)."),
			"self_signed": schema.BoolAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Whether the certificate is self-signed.",
			},
		},
	}
}

func (r *exchangeCertificateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

func (r *exchangeCertificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg exchangeCertificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Write-only values are only in the configuration.
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := certificateID(plan.Server.ValueString(), plan.Thumbprint.ValueString())

	if cfg.PfxBase64.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("pfx_base64_wo"), "Missing PFX",
			fmt.Sprintf("pfx_base64_wo is required to import the certificate. To manage a certificate that is already on the server, "+
				"import it into the Terraform state instead, with the ID %q.", id))
		return
	}
	if !r.client.Persistent() {
		resp.Diagnostics.AddError("Persistent bridge required",
			fmt.Sprintf("Importing a PFX needs the persistent PowerShell bridge: in %q mode, the PFX password would be on the PowerShell "+
				"command line, which other users of the machine can read. Unset %s or set it to %q.",
				client.BridgeModeOneShot, client.EnvBridgeMode, client.BridgeModePersistent))
		return
	}

	params := map[string]string{
		"Action":      "Import",
		"Identity":    id,
		"PfxData":     cfg.PfxBase64.ValueString(),
		"PfxPassword": cfg.PfxPassword.ValueString(),
	}
	putString(params, "FriendlyName", cfg.FriendlyName)
	putBool(params, "PrivateKeyExportable", cfg.PrivateKeyExportable)

	cert, err := runCertificateScript(ctx, r.client, params)
	if err == nil && cert == nil {
		err = fmt.Errorf("the bridge returned no certificate")
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to import the certificate", err.Error())
		return
	}
	plan.apply(*cert)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *exchangeCertificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state exchangeCertificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cert, err := runCertificateScript(ctx, r.client, map[string]string{"Action": "Get", "Identity": state.ID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the certificate", err.Error())
		return
	}
	if cert == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.apply(*cert)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update is only reached when nothing but a write-only value changed: every
// other argument replaces the certificate. There is nothing to send.
func (r *exchangeCertificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan exchangeCertificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.PfxBase64 = types.StringNull()
	plan.PfxPassword = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *exchangeCertificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state exchangeCertificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "exchange_certificate.ps1", map[string]string{"Action": "Delete", "Identity": state.ID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to remove the certificate", err.Error())
	}
}

func (r *exchangeCertificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	server, thumbprint, err := parseCertificateID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server"), server)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("thumbprint"), thumbprint)...)
}
