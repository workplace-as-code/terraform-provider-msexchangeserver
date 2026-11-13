package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

var (
	_ resource.Resource                = &activeSyncDeviceAccessRuleResource{}
	_ resource.ResourceWithImportState = &activeSyncDeviceAccessRuleResource{}
)

// activeSyncCharacteristics are the device properties a rule can match.
var activeSyncCharacteristics = []string{"DeviceType", "DeviceModel", "DeviceOS", "UserAgent", "XMSWLHeader"}

// activeSyncDeviceAccessRuleResource manages one Exchange ActiveSync device
// access rule. A rule has no name of its own: Exchange names it
// "<query string> (<characteristic>)", and only its access level can change.
type activeSyncDeviceAccessRuleResource struct {
	client *client.Client
}

type activeSyncDeviceAccessRuleModel struct {
	ID             types.String `tfsdk:"id"`
	QueryString    types.String `tfsdk:"query_string"`
	Characteristic types.String `tfsdk:"characteristic"`
	AccessLevel    types.String `tfsdk:"access_level"`
}

type activeSyncDeviceAccessRuleAPIResponse struct {
	Name           string `json:"Name"`
	QueryString    string `json:"QueryString"`
	Characteristic string `json:"Characteristic"`
	AccessLevel    string `json:"AccessLevel"`
}

func NewActiveSyncDeviceAccessRuleResource() resource.Resource {
	return &activeSyncDeviceAccessRuleResource{}
}

func (r *activeSyncDeviceAccessRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_activesync_device_access_rule"
}

func (r *activeSyncDeviceAccessRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange ActiveSync device access rule (`New-ActiveSyncDeviceAccessRule`), which allows, " +
			"blocks or quarantines the mobile devices that match a device type, model, operating system or user agent. " +
			"Devices that match no rule get the default access level of `msexchangeserver_activesync_organization_settings`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name Exchange gives the rule: `<query_string> (<characteristic>)`, for example `iPhone (DeviceModel)`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"query_string": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Value the device property must equal, as Exchange reports it in `Get-MobileDevice`, " +
					"for example `iPhone` or `iOS 17.5 21F79`. Wildcards and partial matches are not supported. Changing it creates a new rule.",
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: replace,
			},
			"characteristic": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Device property the rule matches. Valid values: `DeviceType`, `DeviceModel`, `DeviceOS`, " +
					"`UserAgent`, `XMSWLHeader`. Changing it creates a new rule.",
				Validators:    []validator.String{stringvalidator.OneOfCaseInsensitive(activeSyncCharacteristics...)},
				PlanModifiers: replace,
			},
			"access_level": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "What happens to the matching devices. Valid values: `Allow`, `Block`, `Quarantine`.",
				Validators:          []validator.String{stringvalidator.OneOfCaseInsensitive("Allow", "Block", "Quarantine")},
				PlanModifiers:       []planmodifier.String{caseInsensitive()},
			},
		},
	}
}

func (r *activeSyncDeviceAccessRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

// params identifies the rule by its query string and characteristic, which
// never change: the script finds the rule with them.
func (r *activeSyncDeviceAccessRuleResource) params(action string, m activeSyncDeviceAccessRuleModel) map[string]string {
	p := map[string]string{
		"Action":         action,
		"QueryString":    m.QueryString.ValueString(),
		"Characteristic": m.Characteristic.ValueString(),
	}
	putString(p, "AccessLevel", m.AccessLevel)
	return p
}

func (r *activeSyncDeviceAccessRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan activeSyncDeviceAccessRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Create", plan, &resp.State, &resp.Diagnostics)
}

func (r *activeSyncDeviceAccessRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan activeSyncDeviceAccessRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Update", plan, &resp.State, &resp.Diagnostics)
}

func (r *activeSyncDeviceAccessRuleResource) write(ctx context.Context, action string, plan activeSyncDeviceAccessRuleModel, state stateSetter, diags diagAppender) {
	output, err := r.client.RunScript(ctx, "activesync_device_access_rule.ps1", r.params(action, plan))
	if err != nil {
		diags.AddError("Failed to write ActiveSync device access rule", err.Error())
		return
	}
	m, err := activeSyncDeviceAccessRuleFromAPI(plan, output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *activeSyncDeviceAccessRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior activeSyncDeviceAccessRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "activesync_device_access_rule.ps1", map[string]string{
		"Action": "Get", "QueryString": prior.QueryString.ValueString(), "Characteristic": prior.Characteristic.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read ActiveSync device access rule", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	m, err := activeSyncDeviceAccessRuleFromAPI(prior, output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *activeSyncDeviceAccessRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state activeSyncDeviceAccessRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "activesync_device_access_rule.ps1", map[string]string{
		"Action": "Delete", "QueryString": state.QueryString.ValueString(), "Characteristic": state.Characteristic.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to delete ActiveSync device access rule", err.Error())
	}
}

// ImportState accepts the rule name, "<query_string> (<characteristic>)", for
// example "iPhone (DeviceModel)".
func (r *activeSyncDeviceAccessRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	queryString, characteristic, ok := parseActiveSyncRuleName(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf(
			"Expected <query_string> (<characteristic>), for example \"iPhone (DeviceModel)\", got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("query_string"), queryString)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("characteristic"), characteristic)...)
}

// parseActiveSyncRuleName splits "iPhone (DeviceModel)" into its query string
// and characteristic. The query string may itself contain parentheses: only
// the last group is the characteristic.
func parseActiveSyncRuleName(name string) (queryString, characteristic string, ok bool) {
	i := strings.LastIndex(name, " (")
	if i <= 0 || !strings.HasSuffix(name, ")") {
		return "", "", false
	}
	characteristic = name[i+2 : len(name)-1]
	for _, c := range activeSyncCharacteristics {
		if strings.EqualFold(c, characteristic) {
			return name[:i], c, true
		}
	}
	return "", "", false
}

// activeSyncDeviceAccessRuleFromAPI builds the state from the bridge response.
// prior keeps the configured case.
func activeSyncDeviceAccessRuleFromAPI(prior activeSyncDeviceAccessRuleModel, output []byte) (activeSyncDeviceAccessRuleModel, error) {
	var a activeSyncDeviceAccessRuleAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		return prior, err
	}
	m := prior
	m.QueryString = foldString(prior.QueryString, a.QueryString)
	m.Characteristic = foldString(prior.Characteristic, a.Characteristic)
	m.AccessLevel = foldString(prior.AccessLevel, a.AccessLevel)
	m.ID = types.StringValue(a.Name)
	return m, nil
}
