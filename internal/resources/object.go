package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// Generic resources for Exchange objects managed with plain New/Get/Set/Remove
// cmdlets (retention policy tags, ...) and for organization-wide settings managed
// with Get/Set only (transport config, ...).
//
// A resource is described by an objectSpec: its cmdlet noun and a list of
// fields, each mapping a Terraform attribute to a cmdlet parameter. The bridge
// script (object.ps1 by default) receives the values as JSON and calls
// <verb>-<noun>, so adding a simple resource only takes a spec.
//
// Every field is Optional + Computed: an attribute left out of the
// configuration keeps its current Exchange value and is recorded in state
// (see partial.go).

type fieldKind int

const (
	boolField fieldKind = iota
	stringField
	int64Field
	enumField
	listField
	daysField    // Exchange time span, exposed as a number of days
	minutesField // Exchange time span shorter than a day or not a whole number of days, in minutes
	sizeField    // Exchange size (ByteQuantifiedSize), exposed in megabytes; 0 is Unlimited
	limitField   // Exchange count that accepts Unlimited; 0 is Unlimited
)

// field maps one Terraform attribute to one cmdlet parameter and property.
type field struct {
	name       string // Terraform attribute, snake_case
	param      string // cmdlet parameter and object property
	kind       fieldKind
	desc       string
	values     []string // enumField only
	validators []validator.Int64
	forceNew   bool // changing the value replaces the object
}

func boolAttr(name, param, desc string) field {
	return field{name: name, param: param, kind: boolField, desc: desc}
}

func stringAttr(name, param, desc string) field {
	return field{name: name, param: param, kind: stringField, desc: desc}
}

func listAttr(name, param, desc string) field {
	return field{name: name, param: param, kind: listField, desc: desc}
}

// sizeAttr is an Exchange size such as MaxSendSize, written as "<n>MB" and
// read back from "35 MB (36,700,160 bytes)". 0 means Unlimited.
func sizeAttr(name, param, desc string, validators ...validator.Int64) field {
	return field{name: name, param: param, kind: sizeField, desc: desc, validators: validators}
}

// limitAttr is an Exchange count that also accepts Unlimited (for example
// MaxRecipientEnvelopeLimit). 0 means Unlimited.
func limitAttr(name, param, desc string, validators ...validator.Int64) field {
	return field{name: name, param: param, kind: limitField, desc: desc, validators: validators}
}

// minutesAttr is an Exchange time span exposed in minutes, for the settings
// whose value is not always a whole number of days (SafetyNetHoldTime is 2
// days on a Mailbox server, 10 minutes on an Edge Transport server).
func minutesAttr(name, param, desc string, validators ...validator.Int64) field {
	return field{name: name, param: param, kind: minutesField, desc: desc, validators: validators}
}

// daysAttr is an Exchange time span (for example AgeLimitForRetention),
// written and read as a whole number of days.
func daysAttr(name, param, desc string, validators ...validator.Int64) field {
	return field{name: name, param: param, kind: daysField, desc: desc, validators: validators}
}

// enumAttr is a string restricted to values, compared without regard to case.
// The valid values are appended to the description.
func enumAttr(name, param, desc string, values ...string) field {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = "`" + v + "`"
	}
	desc = fmt.Sprintf("%s Valid values: %s (case-insensitive).", desc, strings.Join(quoted, ", "))
	return field{name: name, param: param, kind: enumField, desc: desc, values: values}
}

// objectSpec describes one Exchange object or organization-wide setting.
type objectSpec struct {
	typeName    string // resource name without the provider prefix
	noun        string // cmdlet noun, e.g. "RetentionPolicyTag"
	script      string // bridge script; "" means object.ps1
	singleton   bool   // organization-wide setting: Get/Set only
	edge        bool   // also available on Edge Transport servers
	identity    string // singleton identity ("" calls Get/Set without -Identity)
	description string // first sentence of the resource documentation
	notes       string // appended to the description
	fields      []field
}

func (s objectSpec) scriptName() string {
	if s.script != "" {
		return s.script
	}
	return "object.ps1"
}

func (s objectSpec) singletonID() string {
	if s.identity != "" {
		return s.identity
	}
	return "default"
}

// listQuery lists the objects by name; a organization-wide setting has one ID.
func (s objectSpec) listQuery() listQuery {
	if s.singleton {
		return listQuery{id: s.singletonID()}
	}
	return listQuery{noun: s.noun, property: "Name"}
}

func (s objectSpec) attrTypes() map[string]attr.Type {
	types_ := map[string]attr.Type{"id": types.StringType}
	if !s.singleton {
		types_["name"] = types.StringType
	}
	for _, f := range s.fields {
		types_[f.name] = f.attrType()
	}
	return types_
}

func (f field) attrType() attr.Type {
	switch f.kind {
	case boolField:
		return types.BoolType
	case int64Field, daysField, minutesField, sizeField, limitField:
		return types.Int64Type
	case listField:
		return types.ListType{ElemType: types.StringType}
	default:
		return types.StringType
	}
}

func (f field) schemaAttribute() schema.Attribute {
	switch f.kind {
	case boolField:
		return optionalBool(f.desc)
	case int64Field, daysField, minutesField, sizeField, limitField:
		a := optionalInt64(f.desc, f.validators...)
		if f.forceNew {
			a.PlanModifiers = append(a.PlanModifiers, int64planmodifier.RequiresReplaceIfConfigured())
		}
		return a
	case enumField:
		a := optionalEnum(f.desc, f.values...)
		if f.forceNew {
			a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.RequiresReplaceIfConfigured())
		}
		return a
	case listField:
		return optionalList(f.desc)
	default:
		a := optionalString(f.desc)
		if f.forceNew {
			a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.RequiresReplaceIfConfigured())
		}
		return a
	}
}

// objectResource is the generic resource built from an objectSpec.
type objectResource struct {
	spec   objectSpec
	client *client.Client
}

var (
	_ resource.Resource                = &objectResource{}
	_ resource.ResourceWithImportState = &objectResource{}
)

// newObjectResource returns the factory registered in the provider.
func newObjectResource(spec objectSpec) func() resource.Resource {
	return func() resource.Resource { return &objectResource{spec: spec} }
}

func (r *objectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.spec.typeName
}

func (r *objectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{}
	var desc string
	if r.spec.singleton {
		attrs["id"] = schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: fmt.Sprintf("Always `%s`.", r.singletonID()),
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
		desc = fmt.Sprintf("%s (`Set-%s`). This is a singleton: there is exactly one per organization. "+
			"Only the attributes set in the configuration are managed; the others keep their current value. "+
			"Destroying this resource removes it from Terraform state without changing Exchange.", r.spec.description, r.spec.noun)
	} else {
		attrs["id"] = schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The object name (same as `name`).",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
		attrs["name"] = schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The name. Changing it creates a new object.",
			Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			// Exchange names are case-insensitive: a case-only change is not a new object.
			PlanModifiers: []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()},
		}
		desc = fmt.Sprintf("%s (`New-%s`). Attributes left out of the configuration take the Exchange default when the object "+
			"is created and are not managed afterwards.", r.spec.description, r.spec.noun)
	}
	if r.spec.notes != "" {
		desc += " " + r.spec.notes
	}
	for _, f := range r.spec.fields {
		attrs[f.name] = f.schemaAttribute()
	}
	resp.Schema = schema.Schema{MarkdownDescription: desc, Attributes: attrs}
}

// singletonID is the id recorded in state for a organization-wide setting.
func (r *objectResource) singletonID() string {
	return r.spec.singletonID()
}

func (r *objectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

func (r *objectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	action := "Create"
	if r.spec.singleton {
		action = "Update"
	}
	state, diags := r.write(ctx, action, plan, config, r.identity(plan))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *objectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, r.spec.scriptName(), r.params("Get", r.identity(prior), nil))
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Failed to read %s", r.spec.typeName), err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	state, diags := r.fromAPI(prior, output)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *objectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config, prior types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.write(ctx, "Update", plan, config, r.identity(prior))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Delete removes the object. Organization-wide settings cannot be deleted: they are
// only removed from Terraform state.
func (r *objectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var prior types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.spec.singleton {
		warnStateOnlyDestroy(&resp.Diagnostics, r.spec.typeName, "Organization-wide settings cannot be deleted.")
		return
	}
	if _, err := r.client.RunScript(ctx, r.spec.scriptName(), r.params("Delete", r.identity(prior), nil)); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Failed to delete %s", r.spec.typeName), err.Error())
	}
}

// ImportState accepts the object name. Organization-wide settings accept any ID.
func (r *objectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.spec.singleton {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), r.singletonID())...)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

// identity returns the Exchange identity of the object described by a plan or
// state ("" for a singleton called without -Identity).
func (r *objectResource) identity(o types.Object) string {
	if r.spec.singleton {
		return r.spec.identity
	}
	if id, ok := o.Attributes()["id"].(types.String); ok && !id.IsNull() && !id.IsUnknown() && id.ValueString() != "" {
		return id.ValueString()
	}
	if name, ok := o.Attributes()["name"].(types.String); ok {
		return name.ValueString()
	}
	return ""
}

// params builds the bridge parameters. values holds the cmdlet parameters to
// set; Properties lists every property the script must return.
func (r *objectResource) params(action, identity string, values map[string]any) map[string]string {
	props := make([]string, 0, len(r.spec.fields))
	for _, f := range r.spec.fields {
		props = append(props, f.param)
	}
	p := map[string]string{
		"Action":     action,
		"Noun":       r.spec.noun,
		"Identity":   identity,
		"Properties": encodeJSON(props),
	}
	if values != nil {
		p["Values"] = encodeJSON(values)
	}
	return p
}

// write sends the known plan values to Exchange and returns the resulting state.
// Only the attributes set in the configuration are sent: the plan also holds
// the current value of the others (UseStateForUnknown), and writing them back
// could fail (an empty address) or overwrite a change made in the meantime.
func (r *objectResource) write(ctx context.Context, action string, plan, config types.Object, identity string) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	values := map[string]any{}
	attrs, configured := plan.Attributes(), config.Attributes()
	for _, f := range r.spec.fields {
		v := attrs[f.name]
		if c := configured[f.name]; c == nil || c.IsNull() {
			continue // not in the configuration: not managed
		}
		if v == nil || v.IsNull() || v.IsUnknown() {
			continue // not configured: Exchange keeps its current value
		}
		if f.forceNew && action == "Update" {
			continue // a change replaces the object: Set-<noun> has no such parameter
		}
		if f.kind == daysField {
			// Exchange time spans are written as d.hh:mm:ss; 0 (no limit) as null.
			if days := v.(types.Int64).ValueInt64(); days > 0 {
				values[f.param] = fmt.Sprintf("%d.00:00:00", days)
			} else {
				values[f.param] = nil
			}
			continue
		}
		if f.kind == minutesField {
			// Written as d.hh:mm:ss, the format Exchange parses into a time span.
			m := v.(types.Int64).ValueInt64()
			values[f.param] = fmt.Sprintf("%d.%02d:%02d:00", m/1440, m%1440/60, m%60)
			continue
		}
		if f.kind == sizeField || f.kind == limitField {
			// 0 stands for Unlimited, the value Exchange shows for no limit.
			n := v.(types.Int64).ValueInt64()
			switch {
			case n == 0:
				values[f.param] = "Unlimited"
			case f.kind == sizeField:
				values[f.param] = fmt.Sprintf("%dMB", n)
			default:
				// A string, like the sizes: Exchange parses it into Unlimited<int>.
				values[f.param] = strconv.FormatInt(n, 10)
			}
			continue
		}
		switch tv := v.(type) {
		case types.Bool:
			values[f.param] = tv.ValueBool()
		case types.Int64:
			values[f.param] = tv.ValueInt64()
		case types.String:
			values[f.param] = tv.ValueString()
		case types.List:
			items := []string{}
			diags.Append(tv.ElementsAs(ctx, &items, false)...)
			values[f.param] = items
		}
	}
	if diags.HasError() {
		return plan, diags
	}
	output, err := r.client.RunScript(ctx, r.spec.scriptName(), r.params(action, identity, values))
	if err != nil {
		diags.AddError(fmt.Sprintf("Failed to write %s", r.spec.typeName), err.Error())
		return plan, diags
	}
	state, d := r.fromAPI(plan, output)
	diags.Append(d...)
	return state, diags
}

// fromAPI builds the state from the JSON object returned by the bridge. prior
// is the plan or previous state: it supplies the name and keeps the configured
// case of enum values.
func (r *objectResource) fromAPI(prior types.Object, output []byte) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	var api map[string]any
	if err := json.Unmarshal(output, &api); err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return prior, diags
	}
	priorAttrs := prior.Attributes()
	values := map[string]attr.Value{}

	if r.spec.singleton {
		values["id"] = types.StringValue(r.singletonID())
	} else {
		priorName, _ := priorAttrs["name"].(types.String)
		name := priorName
		if apiName, _ := api["Name"].(string); apiName != "" {
			name = foldString(priorName, apiName)
		}
		values["id"] = name
		values["name"] = name
	}

	for _, f := range r.spec.fields {
		raw := api[f.param]
		switch f.kind {
		case boolField:
			b, _ := raw.(bool)
			values[f.name] = types.BoolValue(b)
		case int64Field:
			n, _ := raw.(float64)
			values[f.name] = types.Int64Value(int64(n))
		case daysField:
			values[f.name] = types.Int64Value(timeSpanDays(raw))
		case minutesField:
			values[f.name] = types.Int64Value(timeSpanMinutes(raw))
		case sizeField:
			values[f.name] = types.Int64Value(sizeMegabytes(raw))
		case limitField:
			values[f.name] = types.Int64Value(unlimitedCount(raw))
		case listField:
			values[f.name] = listValue(toStrings(raw))
		case enumField:
			s, _ := raw.(string)
			priorValue, _ := priorAttrs[f.name].(types.String)
			values[f.name] = foldString(priorValue, s)
		default:
			s, _ := raw.(string)
			values[f.name] = types.StringValue(s)
		}
	}
	obj, d := types.ObjectValue(r.spec.attrTypes(), values)
	diags.Append(d...)
	return obj, diags
}

// timeSpanDays converts an Exchange time span ("30.00:00:00", "Unlimited") or
// a number of days to whole days. Unlimited is 0.
func timeSpanDays(raw any) int64 {
	switch v := raw.(type) {
	case float64:
		return int64(v)
	case string:
		if days, _, ok := strings.Cut(v, "."); ok {
			n, err := strconv.ParseInt(days, 10, 64)
			if err == nil {
				return n
			}
		}
	}
	return 0
}

// timeSpanMinutes converts an Exchange time span ("2.00:00:00", "00:10:00",
// "1.12:30:15.5000000") to whole minutes; seconds are dropped. Unlimited is 0.
func timeSpanMinutes(raw any) int64 {
	s, ok := raw.(string)
	if !ok {
		return 0
	}
	var days int64
	if d, rest, found := strings.Cut(s, "."); found && !strings.Contains(d, ":") {
		n, err := strconv.ParseInt(d, 10, 64)
		if err != nil {
			return 0
		}
		days, s = n, rest
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0
	}
	hours, errH := strconv.ParseInt(parts[0], 10, 64)
	minutes, errM := strconv.ParseInt(parts[1], 10, 64)
	if errH != nil || errM != nil {
		return 0
	}
	return days*1440 + hours*60 + minutes
}

// sizeMegabytes converts an Exchange size to whole megabytes. Exchange returns
// sizes as "35 MB (36,700,160 bytes)", with digit separators that depend on
// the server culture, so only the digits of the byte count are kept.
// Unlimited is 0.
func sizeMegabytes(raw any) int64 {
	switch v := raw.(type) {
	case float64:
		return int64(v) >> 20
	case string:
		if strings.EqualFold(v, "Unlimited") {
			return 0
		}
		if open := strings.LastIndex(v, "("); open >= 0 && strings.HasSuffix(v, "bytes)") {
			var bytes int64
			for _, c := range v[open:] {
				if c >= '0' && c <= '9' {
					bytes = bytes*10 + int64(c-'0')
				}
			}
			return bytes >> 20
		}
		// "35 MB" or "35MB": the number and its unit.
		number := strings.TrimSpace(strings.TrimRightFunc(v, func(c rune) bool { return c < '0' || c > '9' }))
		unit := strings.ToUpper(strings.TrimSpace(v[len(number):]))
		n, err := strconv.ParseInt(number, 10, 64)
		if err != nil {
			return 0
		}
		switch unit {
		case "KB":
			return n >> 10
		case "MB":
			return n
		case "GB":
			return n << 10
		case "TB":
			return n << 20
		default: // bytes
			return n >> 20
		}
	}
	return 0
}

// unlimitedCount converts an Exchange count that accepts Unlimited ("500",
// 500 or "Unlimited") to a number. Unlimited is 0.
func unlimitedCount(raw any) int64 {
	switch v := raw.(type) {
	case float64:
		return int64(v)
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

// toStrings converts a JSON array (or a single value) to strings. PowerShell
// can unwrap a one-element array to its value and an empty array to "", so
// both are accepted.
func toStrings(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return []string{}
	case string:
		if v == "" {
			return []string{}
		}
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return []string{fmt.Sprint(v)}
	}
}

// encodeJSON encodes a value as base64(JSON) for the bridge, so it survives
// the trip through the PowerShell host unchanged.
func encodeJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		// Only maps, slices and scalars are encoded here: this cannot fail.
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// warnStateOnlyDestroy tells the user that destroying the resource removed it
// from the Terraform state only: the object and its settings stay as they are.
func warnStateOnlyDestroy(diags *diag.Diagnostics, typeName, reason string) {
	diags.AddWarning(
		fmt.Sprintf("msexchangeserver_%s was only removed from the Terraform state", typeName),
		reason+" Destroying the resource only removes it from the Terraform state: its settings keep their current values. To undo a change made with Terraform, set the previous value and apply before destroying.",
	)
}
