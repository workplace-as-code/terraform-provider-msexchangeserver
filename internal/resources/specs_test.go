package resources

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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
