package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEncodeStringListKeepsCommasAndQuotes(t *testing.T) {
	in := []string{`invoice, urgent`, `say "hi"`}
	elems := []attr.Value{types.StringValue(in[0]), types.StringValue(in[1])}
	list := types.ListValueMust(types.StringType, elems)

	enc, diags := encodeStringList(context.Background(), list)
	if diags.HasError() {
		t.Fatal(diags)
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0] != in[0] || out[1] != in[1] {
		t.Errorf("round trip mismatch: %v", out)
	}
}

func TestEncodeStringListEmpty(t *testing.T) {
	for _, l := range []types.List{types.ListNull(types.StringType), types.ListValueMust(types.StringType, nil)} {
		if enc, _ := encodeStringList(context.Background(), l); enc != "" {
			t.Errorf("expected empty encoding, got %q", enc)
		}
	}
}

func TestApplyTransportRuleResponseAvoidsSpuriousDiffs(t *testing.T) {
	m := transportRuleModel{
		FromAddressContains:    types.ListNull(types.StringType),
		SubjectContains:        types.ListNull(types.StringType),
		AttachmentIsExecutable: types.BoolNull(),
	}
	applyTransportRuleResponse(&m, transportRuleAPIResponse{Identity: "r", Name: "r", Action: "MoveToJmf"})

	if !m.FromAddressContains.IsNull() || !m.SubjectContains.IsNull() {
		t.Error("unset lists must stay null when Exchange has no values")
	}
	if !m.AttachmentIsExecutable.IsNull() {
		t.Error("unset bool must stay null when Exchange reports false")
	}

	// Drift: someone added a condition in the Exchange admin center.
	applyTransportRuleResponse(&m, transportRuleAPIResponse{Identity: "r", SubjectContains: []string{"added"}})
	if len(m.SubjectContains.Elements()) != 1 {
		t.Error("conditions added outside Terraform must show up in state")
	}
}
