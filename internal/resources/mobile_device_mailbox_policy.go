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

var _ resource.Resource = &mobileDeviceMailboxPolicyResource{}
var _ resource.ResourceWithImportState = &mobileDeviceMailboxPolicyResource{}

type mobileDeviceMailboxPolicyResource struct{ client *client.Client }

type mobileDeviceMailboxPolicyModel struct {
	ID                           types.String `tfsdk:"id"`
	Name                         types.String `tfsdk:"name"`
	IsDefault                    types.Bool   `tfsdk:"is_default"`
	PasswordEnabled              types.Bool   `tfsdk:"password_enabled"`
	AlphanumericPasswordRequired types.Bool   `tfsdk:"alphanumeric_password_required"`
	PasswordRecoveryEnabled      types.Bool   `tfsdk:"password_recovery_enabled"`
	DeviceEncryptionEnabled      types.Bool   `tfsdk:"device_encryption_enabled"`
	AllowSimplePassword          types.Bool   `tfsdk:"allow_simple_password"`
	MaxPasswordFailedAttempts    types.Int64  `tfsdk:"max_password_failed_attempts"`
	MinPasswordLength            types.Int64  `tfsdk:"min_password_length"`
	MaxInactivityTimeLock        types.String `tfsdk:"max_inactivity_time_lock"`
	RequireDeviceEncryption      types.Bool   `tfsdk:"require_device_encryption"`
	AllowStorageCard             types.Bool   `tfsdk:"allow_storage_card"`
	AllowCamera                  types.Bool   `tfsdk:"allow_camera"`
	AllowWiFi                    types.Bool   `tfsdk:"allow_wifi"`
	AllowTextMessaging           types.Bool   `tfsdk:"allow_text_messaging"`
	AllowBluetooth               types.String `tfsdk:"allow_bluetooth"`
	AllowBrowser                 types.Bool   `tfsdk:"allow_browser"`
}

type mobileDeviceMailboxPolicyAPIResponse struct {
	Identity                     string `json:"Identity"`
	Name                         string `json:"Name"`
	IsDefault                    bool   `json:"IsDefault"`
	PasswordEnabled              bool   `json:"PasswordEnabled"`
	AlphanumericPasswordRequired bool   `json:"AlphanumericPasswordRequired"`
	PasswordRecoveryEnabled      bool   `json:"PasswordRecoveryEnabled"`
	DeviceEncryptionEnabled      bool   `json:"DeviceEncryptionEnabled"`
	AllowSimplePassword          bool   `json:"AllowSimplePassword"`
	MaxPasswordFailedAttempts    int64  `json:"MaxPasswordFailedAttempts"`
	MinPasswordLength            int64  `json:"MinPasswordLength"`
	MaxInactivityTimeLock        string `json:"MaxInactivityTimeLock"`
	RequireDeviceEncryption      bool   `json:"RequireDeviceEncryption"`
	AllowStorageCard             bool   `json:"AllowStorageCard"`
	AllowCamera                  bool   `json:"AllowCamera"`
	AllowWiFi                    bool   `json:"AllowWiFi"`
	AllowTextMessaging           bool   `json:"AllowTextMessaging"`
	AllowBluetooth               string `json:"AllowBluetooth"`
	AllowBrowser                 bool   `json:"AllowBrowser"`
}

func NewMobileDeviceMailboxPolicyResource() resource.Resource {
	return &mobileDeviceMailboxPolicyResource{}
}

func (r *mobileDeviceMailboxPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mobile_device_mailbox_policy"
}

func (r *mobileDeviceMailboxPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange Mobile Device Mailbox Policy. Controls security settings for mobile devices connecting via Exchange ActiveSync.",
		Attributes: map[string]schema.Attribute{
			"id":                             schema.StringAttribute{MarkdownDescription: "The unique identifier of the mobile device mailbox policy.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":                           schema.StringAttribute{MarkdownDescription: "Name of the policy. Immutable after creation.", Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"is_default":                     optionalBool("Set this policy as the default mobile device mailbox policy."),
			"password_enabled":               optionalBool("Require a password on the device."),
			"alphanumeric_password_required": optionalBool("Require an alphanumeric password."),
			"password_recovery_enabled":      optionalBool("Enable password recovery."),
			"device_encryption_enabled":      optionalBool("Require device encryption."),
			"allow_simple_password":          optionalBool("Allow simple numeric passwords."),
			"max_password_failed_attempts":   optionalInt64("Failed password attempts before the device is wiped, from 4 to 16. `0` means unlimited."),
			"min_password_length":            optionalInt64("Minimum password length."),
			"max_inactivity_time_lock":       optionalString("Screen lock timeout as a timespan string (e.g. `00:05:00` for 5 minutes)."),
			"require_device_encryption":      optionalBool("Enforce storage card encryption."),
			"allow_storage_card":             optionalBool("Allow removable storage cards."),
			"allow_camera":                   optionalBool("Allow the device camera."),
			"allow_wifi":                     optionalBool("Allow Wi-Fi connections."),
			"allow_text_messaging":           optionalBool("Allow SMS / text messaging."),
			"allow_bluetooth":                optionalString("Bluetooth setting. Valid values: `Allow`, `Disable`, `HandsfreeOnly`."),
			"allow_browser":                  optionalBool("Allow the built-in web browser."),
		},
	}
}

func (r *mobileDeviceMailboxPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *mobileDeviceMailboxPolicyResource) buildParams(action string, cfg mobileDeviceMailboxPolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putBool(p, "IsDefault", cfg.IsDefault)
	putBool(p, "PasswordEnabled", cfg.PasswordEnabled)
	putBool(p, "AlphanumericPasswordRequired", cfg.AlphanumericPasswordRequired)
	putBool(p, "PasswordRecoveryEnabled", cfg.PasswordRecoveryEnabled)
	putBool(p, "DeviceEncryptionEnabled", cfg.DeviceEncryptionEnabled)
	putBool(p, "AllowSimplePassword", cfg.AllowSimplePassword)
	putInt64(p, "MaxPasswordFailedAttempts", cfg.MaxPasswordFailedAttempts)
	putInt64(p, "MinPasswordLength", cfg.MinPasswordLength)
	putString(p, "MaxInactivityTimeLock", cfg.MaxInactivityTimeLock)
	putBool(p, "RequireDeviceEncryption", cfg.RequireDeviceEncryption)
	putBool(p, "AllowStorageCard", cfg.AllowStorageCard)
	putBool(p, "AllowCamera", cfg.AllowCamera)
	putBool(p, "AllowWiFi", cfg.AllowWiFi)
	putBool(p, "AllowTextMessaging", cfg.AllowTextMessaging)
	putString(p, "AllowBluetooth", cfg.AllowBluetooth)
	putBool(p, "AllowBrowser", cfg.AllowBrowser)
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

func (r *mobileDeviceMailboxPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mobileDeviceMailboxPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg mobileDeviceMailboxPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "mobile_device_mailbox_policy.ps1", r.buildParams("Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create mobile device mailbox policy", err.Error())
		return
	}
	var a mobileDeviceMailboxPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, mobileDeviceMailboxPolicyFromAPI(a))...)
}

func (r *mobileDeviceMailboxPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mobileDeviceMailboxPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "mobile_device_mailbox_policy.ps1", map[string]string{"Action": "Get", "Identity": state.ID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read mobile device mailbox policy", err.Error())
		return
	}
	var a mobileDeviceMailboxPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, mobileDeviceMailboxPolicyFromAPI(a))...)
}

func (r *mobileDeviceMailboxPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state mobileDeviceMailboxPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg mobileDeviceMailboxPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "mobile_device_mailbox_policy.ps1", r.buildParams("Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update mobile device mailbox policy", err.Error())
		return
	}
	var a mobileDeviceMailboxPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, mobileDeviceMailboxPolicyFromAPI(a))...)
}

func (r *mobileDeviceMailboxPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mobileDeviceMailboxPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "mobile_device_mailbox_policy.ps1", map[string]string{"Action": "Delete", "Identity": state.ID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete mobile device mailbox policy", err.Error())
	}
}

func (r *mobileDeviceMailboxPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mobileDeviceMailboxPolicyFromAPI(a mobileDeviceMailboxPolicyAPIResponse) mobileDeviceMailboxPolicyModel {
	return mobileDeviceMailboxPolicyModel{
		ID: types.StringValue(a.Identity), Name: types.StringValue(a.Name),
		IsDefault: types.BoolValue(a.IsDefault), PasswordEnabled: types.BoolValue(a.PasswordEnabled),
		AlphanumericPasswordRequired: types.BoolValue(a.AlphanumericPasswordRequired),
		PasswordRecoveryEnabled:      types.BoolValue(a.PasswordRecoveryEnabled),
		DeviceEncryptionEnabled:      types.BoolValue(a.DeviceEncryptionEnabled),
		AllowSimplePassword:          types.BoolValue(a.AllowSimplePassword),
		MaxPasswordFailedAttempts:    types.Int64Value(a.MaxPasswordFailedAttempts),
		MinPasswordLength:            types.Int64Value(a.MinPasswordLength),
		MaxInactivityTimeLock:        types.StringValue(a.MaxInactivityTimeLock),
		RequireDeviceEncryption:      types.BoolValue(a.RequireDeviceEncryption),
		AllowStorageCard:             types.BoolValue(a.AllowStorageCard),
		AllowCamera:                  types.BoolValue(a.AllowCamera),
		AllowWiFi:                    types.BoolValue(a.AllowWiFi),
		AllowTextMessaging:           types.BoolValue(a.AllowTextMessaging),
		AllowBluetooth:               types.StringValue(a.AllowBluetooth),
		AllowBrowser:                 types.BoolValue(a.AllowBrowser),
	}
}
