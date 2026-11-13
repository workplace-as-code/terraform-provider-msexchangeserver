package resources

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var snakeCase = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// TestObjectSpecs checks every generic resource: unique snake_case attributes,
// unique cmdlet parameters, and a schema the framework accepts.
func TestObjectSpecs(t *testing.T) {
	seen := map[string]bool{}
	for _, spec := range objectSpecs {
		if seen[spec.typeName] {
			t.Errorf("duplicate resource %s", spec.typeName)
		}
		seen[spec.typeName] = true

		names, params := map[string]bool{"id": true, "name": true}, map[string]bool{}
		for _, f := range spec.fields {
			if !snakeCase.MatchString(f.name) || names[f.name] {
				t.Errorf("%s: invalid or duplicate attribute %q", spec.typeName, f.name)
			}
			if params[f.param] {
				t.Errorf("%s: parameter %s mapped twice", spec.typeName, f.param)
			}
			if f.kind == enumField && len(f.values) == 0 {
				t.Errorf("%s.%s: enum without values", spec.typeName, f.name)
			}
			if f.readOnly && f.forceNew {
				t.Errorf("%s.%s: a read-only attribute cannot replace the object", spec.typeName, f.name)
			}
			names[f.name], params[f.param] = true, true
		}

		r := newObjectResource(spec)()
		var resp resource.SchemaResponse
		r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
		resp.Diagnostics.Append(resp.Schema.ValidateImplementation(context.Background())...)
		if resp.Diagnostics.HasError() {
			t.Errorf("%s: invalid schema: %v", spec.typeName, resp.Diagnostics)
		}
	}
}

func TestFromAPIConvertsTimeSpansAndKeepsCase(t *testing.T) {
	r := &objectResource{spec: objectSpecs[0]} // retention_policy_tag
	plan, diags := r.fromAPI(mustObject(t, r, map[string]string{"name": "Archive", "retention_action": "movetoarchive"}),
		[]byte(`{"Name":"Archive","RetentionAction":"MoveToArchive","AgeLimitForRetention":"730.00:00:00","RetentionEnabled":true}`))
	if diags.HasError() {
		t.Fatal(diags)
	}
	attrs := plan.Attributes()
	if got := attrs["retention_action"].String(); got != `"movetoarchive"` {
		t.Errorf("retention_action = %s, want the configured case", got)
	}
	if got := attrs["age_limit_for_retention"].String(); got != "730" {
		t.Errorf("age_limit_for_retention = %s, want 730 days", got)
	}
}

func TestTimeSpanDays(t *testing.T) {
	for in, want := range map[any]int64{"30.00:00:00": 30, "Unlimited": 0, "12:00:00": 0, float64(7): 7, nil: 0} {
		if got := timeSpanDays(in); got != want {
			t.Errorf("timeSpanDays(%v) = %d, want %d", in, got, want)
		}
	}
}

// mustObject builds a plan object for r with the given string attributes set
// and every other attribute null.
func mustObject(t *testing.T, r *objectResource, values map[string]string) types.Object {
	t.Helper()
	attrs := map[string]attr.Value{}
	for name, typ := range r.spec.attrTypes() {
		if v, ok := values[name]; ok {
			attrs[name] = types.StringValue(v)
			continue
		}
		switch typ {
		case types.BoolType:
			attrs[name] = types.BoolNull()
		case types.Int64Type:
			attrs[name] = types.Int64Null()
		case types.StringType:
			attrs[name] = types.StringNull()
		default:
			attrs[name] = types.ListNull(types.StringType)
		}
	}
	obj, diags := types.ObjectValue(r.spec.attrTypes(), attrs)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return obj
}

func TestToStrings(t *testing.T) {
	cases := map[string]struct {
		raw  any
		want int
	}{
		"null":            {nil, 0},
		"empty string":    {"", 0},
		"unwrapped value": {"contoso.com", 1},
		"array":           {[]any{"a", "b"}, 2},
	}
	for name, tc := range cases {
		if got := toStrings(tc.raw); len(got) != tc.want {
			t.Errorf("%s: toStrings(%v) = %v, want %d elements", name, tc.raw, got, tc.want)
		}
	}
}

func TestSizeMegabytes(t *testing.T) {
	for in, want := range map[any]int64{
		"35 MB (36,700,160 bytes)":       35,
		"35 MB (36 700 160 bytes)":       35,
		"35 MB (36.700.160 bytes)":       35,
		"1.953 GB (2,097,152,000 bytes)": 2000,
		"Unlimited":                      0,
		"10MB":                           10,
		"10240 KB":                       10,
		"2 GB":                           2048,
		float64(36700160):                35,
		nil:                              0,
	} {
		if got := sizeMegabytes(in); got != want {
			t.Errorf("sizeMegabytes(%v) = %d, want %d", in, got, want)
		}
	}
}

func TestUnlimitedCount(t *testing.T) {
	for in, want := range map[any]int64{"500": 500, "Unlimited": 0, float64(500): 500, nil: 0} {
		if got := unlimitedCount(in); got != want {
			t.Errorf("unlimitedCount(%v) = %d, want %d", in, got, want)
		}
	}
}

func TestFromAPIConvertsTransportLimits(t *testing.T) {
	var r *objectResource
	for _, spec := range objectSpecs {
		if spec.typeName == "transport_config" {
			r = &objectResource{spec: spec}
		}
	}
	plan, diags := r.fromAPI(mustObject(t, r, nil), []byte(`{"Name":"","MaxSendSize":"35 MB (36,700,160 bytes)",`+
		`"MaxReceiveSize":"Unlimited","MaxRecipientEnvelopeLimit":"500","SafetyNetHoldTime":"2.00:00:00",`+
		`"InternalSMTPServers":["10.0.0.10","10.0.1.0/24"],"TLSSendDomainSecureList":""}`))
	if diags.HasError() {
		t.Fatal(diags)
	}
	attrs := plan.Attributes()
	for name, want := range map[string]string{
		"max_send_size_mb":             "35",
		"max_receive_size_mb":          "0",
		"max_recipient_envelope_limit": "500",
		"safety_net_hold_time_minutes": "2880",
		"internal_smtp_servers":        `["10.0.0.10","10.0.1.0/24"]`,
		"tls_send_domain_secure_list":  "[]",
	} {
		if got := attrs[name].String(); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
}

func TestTimeSpanMinutes(t *testing.T) {
	for in, want := range map[any]int64{
		"2.00:00:00": 2880, "00:10:00": 10, "1.12:30:15.5000000": 2190, "12:00:00": 720,
		"Unlimited": 0, "": 0, nil: 0,
	} {
		if got := timeSpanMinutes(in); got != want {
			t.Errorf("timeSpanMinutes(%v) = %d, want %d", in, got, want)
		}
	}
}

// TestAdoptedNameDescription: renaming an adopted object adopts another one;
// nothing is created.
func TestAdoptedNameDescription(t *testing.T) {
	for _, spec := range objectSpecs {
		if !spec.adopted {
			continue
		}
		var resp resource.SchemaResponse
		(&objectResource{spec: spec}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
		name, ok := resp.Schema.Attributes["name"]
		if !ok {
			continue
		}
		if desc := name.GetMarkdownDescription(); strings.Contains(desc, "creates a new object") ||
			!strings.Contains(desc, "adopts another object") {
			t.Errorf("%s: name description %q", spec.typeName, desc)
		}
	}
}

// A log age of 0 means no age limit: Exchange takes 00:00:00 and refuses
// $null, unlike the other time spans, where 0 is written as $null.
func TestZeroTimeSpanIsWrittenAsZero(t *testing.T) {
	r := &objectResource{spec: objectSpec{typeName: "x", noun: "X", adopted: true, fields: []field{
		logAgeAttr("connectivity_log_max_age_days", "ConnectivityLogMaxAge", "d"),
		daysAttr("other_days", "OtherDays", "d"),
	}}}
	for days, want := range map[int64]any{0: "00:00:00", 45: "45.00:00:00"} {
		plan, diags := types.ObjectValue(r.spec.attrTypes(), map[string]attr.Value{
			"id": types.StringNull(), "name": types.StringValue("EX01"),
			"connectivity_log_max_age_days": types.Int64Value(days), "other_days": types.Int64Value(0),
		})
		if diags.HasError() {
			t.Fatal(diags)
		}
		got, diags := r.writeValues(context.Background(), "Update", plan, plan)
		if diags.HasError() {
			t.Fatal(diags)
		}
		if got["ConnectivityLogMaxAge"] != want {
			t.Errorf("%d days: ConnectivityLogMaxAge = %v, want %v", days, got["ConnectivityLogMaxAge"], want)
		}
		if v, ok := got["OtherDays"]; !ok || v != nil {
			t.Errorf("OtherDays = %v (sent %v), want nil", v, ok)
		}
	}
	if got := timeSpanDays("00:00:00"); got != 0 {
		t.Errorf("timeSpanDays(00:00:00) = %d, want 0", got)
	}
}

func TestTransportServiceSpecs(t *testing.T) {
	for typeName, edge := range map[string]bool{"transport_service": true, "frontend_transport_service": false} {
		s := specByType(t, typeName)
		if !s.adopted || s.scriptName() != "server_settings.ps1" || s.edge != edge {
			t.Errorf("%s: adopted %v, script %s, edge %v; want adopted, server_settings.ps1, edge %v",
				typeName, s.adopted, s.scriptName(), s.edge, edge)
		}
		// The Identity of every Front End Transport service is "Frontend".
		if q := s.listQuery(); q.property != "Name" {
			t.Errorf("%s: listed by %q, want Name", typeName, q.property)
		}
		for _, f := range s.fields {
			if strings.HasSuffix(f.name, "_max_age_days") && !f.zeroTimeSpan {
				t.Errorf("%s.%s: a log age must write 0 as 00:00:00", typeName, f.name)
			}
			if strings.HasSuffix(f.name, "_log_path") && !f.nonEmpty {
				t.Errorf("%s.%s: an empty log path would disable the log with event log errors", typeName, f.name)
			}
			if f.name == "message_tracking_log_subject_logging_enabled" {
				t.Errorf("%s: subject logging is out of scope", typeName)
			}
			if typeName == "frontend_transport_service" && strings.HasPrefix(f.name, "message_tracking_") {
				t.Errorf("%s.%s: the Front End Transport service has no message tracking", typeName, f.name)
			}
		}
	}
}

// A log path cannot be empty: the script would send $null, which disables the
// log and makes Exchange log errors.
func TestPathAttrRefusesEmpty(t *testing.T) {
	a, ok := pathAttr("connectivity_log_path", "ConnectivityLogPath", "d").schemaAttribute().(schema.StringAttribute)
	if !ok {
		t.Fatal("pathAttr is not a string attribute")
	}
	req := validator.StringRequest{ConfigValue: types.StringValue("")}
	for _, v := range a.Validators {
		resp := &validator.StringResponse{}
		v.ValidateString(context.Background(), req, resp)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	t.Error("an empty path passes validation")
}

func TestPopImapSpecs(t *testing.T) {
	for _, typeName := range []string{"pop_settings", "imap_settings"} {
		s := specByType(t, typeName)
		if !s.adopted || s.edge || s.scriptName() != "server_settings.ps1" || s.listQuery().property != "Name" {
			t.Errorf("%s: adopted %v, edge %v, script %s, listed by %q; want adopted, Mailbox only, server_settings.ps1, Name",
				typeName, s.adopted, s.edge, s.scriptName(), s.listQuery().property)
		}
		var loginType *field
		for i := range s.fields {
			if s.fields[i].name == "login_type" {
				loginType = &s.fields[i]
			}
		}
		if loginType == nil || strings.Join(loginType.values, ",") != "PlainTextLogin,PlainTextAuthentication,SecureLogin" {
			t.Errorf("%s: login_type = %+v", typeName, loginType)
		}
	}
}

func TestAdminAuditLogConfigSpec(t *testing.T) {
	s := specByType(t, "admin_audit_log_config")
	if !s.singleton || s.edge || s.edgeOnly || s.identity != "" {
		t.Errorf("singleton %v, edge %v/%v, identity %q: want a Mailbox singleton read without -Identity",
			s.singleton, s.edge, s.edgeOnly, s.identity)
	}
	for _, f := range s.fields {
		switch f.name {
		case "admin_audit_log_age_limit_days":
			// Exchange purges the whole audit log at 0.
			for value, wantError := range map[int64]bool{0: true, 1: false, 24855: false} {
				resp := &validator.Int64Response{}
				for _, v := range f.validators {
					v.ValidateInt64(context.Background(), validator.Int64Request{ConfigValue: types.Int64Value(value)}, resp)
				}
				if resp.Diagnostics.HasError() != wantError {
					t.Errorf("age limit %d: error %v, want %v", value, resp.Diagnostics.HasError(), wantError)
				}
			}
		case "admin_audit_log_cmdlets", "admin_audit_log_parameters", "admin_audit_log_excluded_cmdlets":
			if f.kind != setField {
				t.Errorf("%s is not a set", f.name)
			}
		}
	}
}

// Binding a certificate to POP3 or IMAP4 (exchange_certificate_services)
// changes X509CertificateName: when it is not configured, an update of the
// settings plans it unknown, or the apply that does both would find another
// value than planned.
func TestCertificateNameUnknownOnUpdate(t *testing.T) {
	spec := specByType(t, "pop_settings")
	state := map[string]attr.Value{
		"x509_certificate_name": types.StringValue("tf-ci-cert.example"), "max_connections_per_user": types.Int64Value(17),
	}
	plan := map[string]attr.Value{
		"x509_certificate_name": types.StringValue("tf-ci-cert.example"), "max_connections_per_user": types.Int64Value(16),
	}
	config := map[string]attr.Value{"max_connections_per_user": types.Int64Value(16)}
	got := changedElsewhereUnknown(spec.fields, plan, state, config)
	if !got["x509_certificate_name"].IsUnknown() || got["max_connections_per_user"].IsUnknown() {
		t.Errorf("got %v: want x509_certificate_name unknown, max_connections_per_user kept", got)
	}

	// No change: the plan stays as it is.
	plan["max_connections_per_user"] = types.Int64Value(17)
	if got := changedElsewhereUnknown(spec.fields, plan, state, config); got["x509_certificate_name"].IsUnknown() {
		t.Error("x509_certificate_name became unknown without a change")
	}

	// Configured: Terraform manages it, it keeps the planned value.
	plan["max_connections_per_user"] = types.Int64Value(16)
	config["x509_certificate_name"] = types.StringValue("mail.contoso.com")
	plan["x509_certificate_name"] = types.StringValue("mail.contoso.com")
	if got := changedElsewhereUnknown(spec.fields, plan, state, config); got["x509_certificate_name"].IsUnknown() {
		t.Error("a configured x509_certificate_name became unknown")
	}
}

// A log age under a day (12:00:00, set by hand) is a limit: it reads as 1 day,
// never as 0, which means no age limit and would remove it on the next write.
func TestLogAgeUnderADayReadsAsOneDay(t *testing.T) {
	r := &objectResource{spec: objectSpec{typeName: "x", noun: "X", adopted: true, fields: []field{
		logAgeAttr("connectivity_log_max_age_days", "ConnectivityLogMaxAge", "d"),
	}}}
	for raw, want := range map[string]string{"12:00:00": "1", "00:00:00": "0", "30.00:00:00": "30"} {
		got, diags := r.fromAPI(mustObject(t, r, map[string]string{"name": "EX01"}),
			[]byte(`{"Name":"EX01","ConnectivityLogMaxAge":"`+raw+`"}`))
		if diags.HasError() {
			t.Fatal(diags)
		}
		if v := got.Attributes()["connectivity_log_max_age_days"].String(); v != want {
			t.Errorf("%s reads as %s days, want %s", raw, v, want)
		}
	}
}

func TestJournalRuleSpec(t *testing.T) {
	s := specByType(t, "journal_rule")
	if s.scriptName() != "journal_rule.ps1" || s.singleton || s.adopted || s.edge {
		t.Errorf("script %s, singleton %v, adopted %v, edge %v: want a Mailbox object on journal_rule.ps1",
			s.scriptName(), s.singleton, s.adopted, s.edge)
	}
	names := map[string]field{}
	for _, f := range s.fields {
		names[f.name] = f
	}
	for _, n := range []string{"journal_email_address", "recipient", "scope", "enabled"} {
		if _, ok := names[n]; !ok {
			t.Errorf("no field %s", n)
		}
	}
	if got := strings.Join(names["scope"].values, ","); got != "Global,Internal,External" {
		t.Errorf("scope values %s", got)
	}
}

func TestActiveSyncOrganizationSettingsSpec(t *testing.T) {
	s := specByType(t, "activesync_organization_settings")
	if !s.singleton || s.edge || s.edgeOnly {
		t.Errorf("singleton %v, edge %v/%v: want a Mailbox singleton", s.singleton, s.edge, s.edgeOnly)
	}
	for _, f := range s.fields {
		if f.name == "default_access_level" && strings.Join(f.values, ",") != "Allow,Block,Quarantine" {
			t.Errorf("default_access_level values %v", f.values)
		}
	}
}

// The device access rules are a hand-written resource, registered with their
// list query and kept off Edge.
func TestActiveSyncDeviceAccessRuleRegistered(t *testing.T) {
	for _, reg := range registrations() {
		var resp resource.MetadataResponse
		reg.newResource().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "msexchangeserver"}, &resp)
		if resp.TypeName == "msexchangeserver_activesync_device_access_rule" {
			if reg.edge || reg.list.noun != "ActiveSyncDeviceAccessRule" || reg.list.property != "Name" {
				t.Errorf("registration %+v", reg)
			}
			return
		}
	}
	t.Error("msexchangeserver_activesync_device_access_rule is not registered")
}
