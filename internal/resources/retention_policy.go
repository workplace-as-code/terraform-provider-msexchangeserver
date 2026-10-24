package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

var _ resource.Resource = &retentionPolicyResource{}
var _ resource.ResourceWithImportState = &retentionPolicyResource{}

type retentionPolicyResource struct{ client *client.Client }

type retentionPolicyModel struct {
	ID                          types.String `tfsdk:"id"`
	Name                        types.String `tfsdk:"name"`
	RetentionPolicyTagLinks     types.Set    `tfsdk:"retention_policy_tag_links"`
	IsDefault                   types.Bool   `tfsdk:"is_default"`
	IsDefaultArbitrationMailbox types.Bool   `tfsdk:"is_default_arbitration_mailbox"`
}

type retentionPolicyAPIResponse struct {
	Identity                    string          `json:"Identity"`
	Name                        string          `json:"Name"`
	RetentionPolicyTagLinks     FlexStringSlice `json:"RetentionPolicyTagLinks"`
	IsDefault                   bool            `json:"IsDefault"`
	IsDefaultArbitrationMailbox bool            `json:"IsDefaultArbitrationMailbox"`
}

func NewRetentionPolicyResource() resource.Resource { return &retentionPolicyResource{} }

func (r *retentionPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_retention_policy"
}

func (r *retentionPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange Retention Policy. A retention policy is a container that groups retention policy tags and is applied to mailboxes to enforce messaging records management (MRM).",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{MarkdownDescription: "The unique identifier of the retention policy.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{MarkdownDescription: "Name of the retention policy. Immutable after creation.", Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"retention_policy_tag_links": schema.SetAttribute{
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
				ElementType:   types.StringType, Optional: true, Computed: true,
				MarkdownDescription: "Names of the retention policy tags linked to this policy.",
			},
			"is_default": optionalBool("Set this policy as the default retention policy applied to new mailboxes."),
			"is_default_arbitration_mailbox": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this is the default retention policy for arbitration mailboxes (read-only).",
			},
		},
	}
}

func (r *retentionPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *retentionPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var cfg retentionPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "retention_policy.ps1", retentionPolicyParams(ctx, "Create", "", cfg))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create retention policy", err.Error())
		return
	}
	var a retentionPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, retentionPolicyFromAPI(a))...)
}

func (r *retentionPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state retentionPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "retention_policy.ps1", map[string]string{"Action": "Get", "Identity": state.ID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read retention policy", err.Error())
		return
	}
	var a retentionPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, retentionPolicyFromAPI(a))...)
}

func (r *retentionPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var cfg, state retentionPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "retention_policy.ps1", retentionPolicyParams(ctx, "Update", state.ID.ValueString(), cfg))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update retention policy", err.Error())
		return
	}
	var a retentionPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, retentionPolicyFromAPI(a))...)
}

func (r *retentionPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state retentionPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "retention_policy.ps1", map[string]string{"Action": "Delete", "Identity": state.ID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete retention policy", err.Error())
	}
}

func (r *retentionPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func retentionPolicyFromAPI(a retentionPolicyAPIResponse) retentionPolicyModel {
	return retentionPolicyModel{
		ID:                          types.StringValue(a.Identity),
		Name:                        types.StringValue(a.Name),
		RetentionPolicyTagLinks:     stringsToSet(a.RetentionPolicyTagLinks),
		IsDefault:                   types.BoolValue(a.IsDefault),
		IsDefaultArbitrationMailbox: types.BoolValue(a.IsDefaultArbitrationMailbox),
	}
}

// retentionPolicyParams sends only the attributes set in the configuration.
func retentionPolicyParams(ctx context.Context, action, identity string, cfg retentionPolicyModel) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	if identity != "" {
		p["Identity"] = identity
	}
	if known(cfg.RetentionPolicyTagLinks) {
		p["RetentionPolicyTagLinks"] = setToCSV(ctx, cfg.RetentionPolicyTagLinks)
	}
	putBool(p, "IsDefault", cfg.IsDefault)
	return p
}
