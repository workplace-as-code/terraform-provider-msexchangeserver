package resources

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// The services of a certificate are only ever added: Exchange has no command
// to take a service away from a certificate, only to enable it on another
// one. The state holds the configured services that the certificate carries,
// not all of them: Exchange adds SMTP to certificates on its own, which would
// otherwise show a difference on every plan.

var (
	_ resource.Resource                = &exchangeCertificateServicesResource{}
	_ resource.ResourceWithImportState = &exchangeCertificateServicesResource{}
)

// certificateServices are the services the resource enables.
var certificateServices = []string{"IIS", "SMTP", "IMAP", "POP"}

type exchangeCertificateServicesResource struct {
	client *client.Client
}

type exchangeCertificateServicesModel struct {
	ID         types.String `tfsdk:"id"`
	Server     types.String `tfsdk:"server"`
	Thumbprint types.String `tfsdk:"thumbprint"`
	Services   types.Set    `tfsdk:"services"`
}

func NewExchangeCertificateServicesResource() resource.Resource {
	return &exchangeCertificateServicesResource{}
}

func (r *exchangeCertificateServicesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exchange_certificate_services"
}

func (r *exchangeCertificateServicesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Enables an Exchange certificate for services (`Enable-ExchangeCertificate`): IIS (HTTPS), SMTP, IMAP and POP. " +
			"Exchange cannot take a service away from a certificate: a service removed from `services`, or a destroy, leaves the " +
			"certificate enabled for it. To move a service, enable it on the new certificate.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "The identity of the certificate, `<server>\\<thumbprint>`.",
			},
			"server":     certificateServerAttribute(),
			"thumbprint": thumbprintAttribute(),
			"services": schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(certificateServices...)),
				},
				MarkdownDescription: "The services to enable the certificate for: `IIS`, `SMTP`, `IMAP`, `POP`. " +
					"Enabling SMTP makes the certificate the default SMTP certificate of the server, which replaces the previous one. " +
					"Services that the certificate carries but that are not listed here are left as they are.",
			},
		},
	}
}

func (r *exchangeCertificateServicesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

// enable enables the planned services and returns the state to save.
func (r *exchangeCertificateServicesResource) enable(ctx context.Context, plan exchangeCertificateServicesModel) (exchangeCertificateServicesModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var wanted []string
	diags.Append(plan.Services.ElementsAs(ctx, &wanted, false)...)
	if diags.HasError() {
		return plan, diags
	}
	id := certificateID(plan.Server.ValueString(), plan.Thumbprint.ValueString())
	cert, err := runCertificateScript(ctx, r.client, map[string]string{
		"Action":   "EnableServices",
		"Identity": id,
		"Services": strings.Join(wanted, ","),
	})
	if err == nil && cert == nil {
		err = fmt.Errorf("the bridge returned no certificate")
	}
	if err != nil {
		diags.AddError("Failed to enable the certificate for services", err.Error())
		return plan, diags
	}
	if missing := missingServices(wanted, cert.Services); len(missing) > 0 {
		diags.AddError("Services not enabled",
			fmt.Sprintf("Exchange did not enable the certificate %s for %s.", id, strings.Join(missing, ", ")))
		return plan, diags
	}
	plan.ID = types.StringValue(id)
	return plan, diags
}

func (r *exchangeCertificateServicesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan exchangeCertificateServicesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.enable(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *exchangeCertificateServicesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state exchangeCertificateServicesModel
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

	// After an import, the state has no services yet: take all of those the
	// resource manages.
	managed := certificateServices
	if !state.Services.IsNull() {
		resp.Diagnostics.Append(state.Services.ElementsAs(ctx, &managed, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	kept := keptServices(managed, cert.Services)
	if state.Services.IsNull() && len(kept) == 0 {
		// A certificate without services has nothing to manage: it is not
		// listed by terraform query, and terraform import reports it missing.
		resp.State.RemoveResource(ctx)
		return
	}
	state.Services = stringsToSet(kept)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *exchangeCertificateServicesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior exchangeCertificateServicesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.enable(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var wanted, before []string
	resp.Diagnostics.Append(plan.Services.ElementsAs(ctx, &wanted, false)...)
	resp.Diagnostics.Append(prior.Services.ElementsAs(ctx, &before, false)...)
	if removed := missingServices(before, wanted); len(removed) > 0 {
		resp.Diagnostics.AddWarning("Services left enabled",
			fmt.Sprintf("The certificate %s is still enabled for %s: Exchange cannot take a service away from a certificate. "+
				"Enable the service on another certificate to move it.", state.ID.ValueString(), strings.Join(removed, ", ")))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *exchangeCertificateServicesResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	warnStateOnlyDestroy(&resp.Diagnostics, "exchange_certificate_services",
		"Exchange cannot take a service away from a certificate: enable the service on another certificate to move it.")
}

func (r *exchangeCertificateServicesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	server, thumbprint, err := parseCertificateID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server"), server)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("thumbprint"), thumbprint)...)
}

// keptServices returns the managed services that the certificate carries.
func keptServices(managed, carried []string) []string {
	var kept []string
	for _, s := range managed {
		if slices.Contains(carried, s) && slices.Contains(certificateServices, s) {
			kept = append(kept, s)
		}
	}
	return kept
}

// missingServices returns the wanted services that are not in got.
func missingServices(wanted, got []string) []string {
	var missing []string
	for _, s := range wanted {
		if !slices.Contains(got, s) {
			missing = append(missing, s)
		}
	}
	return missing
}
