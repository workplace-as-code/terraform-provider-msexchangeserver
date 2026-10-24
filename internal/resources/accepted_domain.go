package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

var _ resource.Resource = &acceptedDomainResource{}
var _ resource.ResourceWithImportState = &acceptedDomainResource{}

type acceptedDomainResource struct{ client *client.Client }

type acceptedDomainModel struct {
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

func NewAcceptedDomainResource() resource.Resource { return &acceptedDomainResource{} }

func (r *acceptedDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_accepted_domain"
}

func (r *acceptedDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an accepted domain of the Exchange organization (`New-AcceptedDomain`). The default accepted domain cannot be removed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{MarkdownDescription: "The unique identifier of the accepted domain.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Friendly name for the accepted domain entry."},
			"domain_name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "The SMTP domain (e.g. `contoso.com`)."},
			"domain_type": optionalEnum("Type of accepted domain: `Authoritative` (used when omitted), `InternalRelay` or `ExternalRelay`.", "Authoritative", "InternalRelay", "ExternalRelay"),
			"default": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this is the default accepted domain (read-only).",
			},
			"match_sub_domains": optionalBool("Also accept mail for all subdomains of this domain."),
		},
	}
}

func (r *acceptedDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *acceptedDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var cfg acceptedDomainModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := map[string]string{"Action": "Create", "Name": cfg.Name.ValueString(), "DomainName": cfg.DomainName.ValueString()}
	putString(p, "DomainType", cfg.DomainType)
	putBool(p, "MatchSubDomains", cfg.MatchSubDomains)
	output, err := r.client.RunScript(ctx, "accepted_domain.ps1", p)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create accepted domain", err.Error())
		return
	}
	var a acceptedDomainAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, acceptedDomainFromAPI(a))...)
}

func (r *acceptedDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state acceptedDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "accepted_domain.ps1", map[string]string{
		"Action": "Get", "Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read accepted domain", err.Error())
		return
	}
	var a acceptedDomainAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, acceptedDomainFromAPI(a))...)
}

func (r *acceptedDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var cfg, state acceptedDomainModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := map[string]string{"Action": "Update", "Identity": state.ID.ValueString()}
	putString(p, "DomainType", cfg.DomainType)
	putBool(p, "MatchSubDomains", cfg.MatchSubDomains)
	output, err := r.client.RunScript(ctx, "accepted_domain.ps1", p)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update accepted domain", err.Error())
		return
	}
	var a acceptedDomainAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, acceptedDomainFromAPI(a))...)
}

func (r *acceptedDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state acceptedDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "accepted_domain.ps1", map[string]string{
		"Action": "Delete", "Identity": state.ID.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to delete accepted domain", err.Error())
		return
	}
}

func (r *acceptedDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func acceptedDomainFromAPI(a acceptedDomainAPIResponse) acceptedDomainModel {
	return acceptedDomainModel{
		ID:              types.StringValue(a.Identity),
		Name:            types.StringValue(a.Name),
		DomainName:      types.StringValue(a.DomainName),
		DomainType:      types.StringValue(a.DomainType),
		Default:         types.BoolValue(a.Default),
		MatchSubDomains: types.BoolValue(a.MatchSubDomains),
	}
}
