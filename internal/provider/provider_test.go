package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEnvOrConfigPrefersExplicitConfig(t *testing.T) {
	t.Setenv("EXCHANGE_TENANT_ID", "from-env")

	if got := envOrConfig("EXCHANGE_TENANT_ID", types.StringValue("from-hcl")); got != "from-hcl" {
		t.Errorf("config should win, got %q", got)
	}
	if got := envOrConfig("EXCHANGE_TENANT_ID", types.StringNull()); got != "from-env" {
		t.Errorf("env should be the fallback, got %q", got)
	}
	if got := envOrConfig("EXCHANGE_TENANT_ID", types.StringUnknown()); got != "from-env" {
		t.Errorf("unknown config should fall back to env, got %q", got)
	}
}

func TestNormalizeRole(t *testing.T) {
	for in, want := range map[string]string{"": "Mailbox", "mailbox": "Mailbox", "EDGE": "Edge", " Edge ": "Edge", "hub": ""} {
		if got := normalizeRole(in); got != want {
			t.Errorf("normalizeRole(%q) = %q, want %q", in, got, want)
		}
	}
}
