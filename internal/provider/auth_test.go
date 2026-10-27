package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// authEnv lists every variable resolveConfig reads. Each case starts with all
// of them empty, so the shell running the tests cannot change the result.
var authEnv = []string{"EXCHANGE_SERVER", "EXCHANGE_USERNAME", "EXCHANGE_PASSWORD", "EXCHANGE_ROLE", "EXCHANGE_AUTH_METHOD"}

// secretPassword must never appear in a diagnostic.
const secretPassword = "pw-secret"

// model builds a provider block from attribute values; absent keys are null.
func model(attrs map[string]string) exchangeProviderModel {
	v := func(name string) types.String {
		if s, ok := attrs[name]; ok {
			return types.StringValue(s)
		}
		return types.StringNull()
	}
	return exchangeProviderModel{
		Server:     v("server"),
		Username:   v("username"),
		Password:   v("password"),
		Role:       v("role"),
		AuthMethod: v("auth_method"),
	}
}

func TestResolveConfig(t *testing.T) {
	server := map[string]string{"server": "exch01.contoso.local"}
	withUser := map[string]string{"server": "exch01.contoso.local", "username": "CONTOSO\\svc", "password": secretPassword}
	integrated := map[string]string{"server": "exch01.contoso.local", "auth_method": "integrated"}

	cases := map[string]struct {
		attrs      map[string]string
		env        map[string]string
		wantError  string // substring of a diagnostic summary, "" when the configuration is valid
		wantMethod string
	}{
		"password by default": {attrs: withUser, wantMethod: client.AuthPassword},
		"password without username": {
			attrs:     map[string]string{"server": "exch01.contoso.local", "password": secretPassword},
			wantError: "Missing username",
		},
		"integrated": {attrs: integrated, wantMethod: client.AuthIntegrated},
		"Integrated from the environment": {
			attrs:      server,
			env:        map[string]string{"EXCHANGE_AUTH_METHOD": "Integrated"},
			wantMethod: client.AuthIntegrated,
		},
		"integrated with a username": {
			attrs:     map[string]string{"server": "exch01.contoso.local", "auth_method": "integrated", "username": "CONTOSO\\svc"},
			wantError: "Conflicting authentication settings",
		},
		"integrated with EXCHANGE_USERNAME": {
			attrs:     integrated,
			env:       map[string]string{"EXCHANGE_USERNAME": "CONTOSO\\svc"},
			wantError: "Conflicting authentication settings",
		},
		"integrated with EXCHANGE_PASSWORD": {
			attrs:     integrated,
			env:       map[string]string{"EXCHANGE_PASSWORD": secretPassword},
			wantError: "Conflicting authentication settings",
		},
		"integrated on Edge": {
			attrs:     map[string]string{"auth_method": "integrated", "role": "Edge"},
			wantError: "Integrated authentication not available on Edge",
		},
		"integrated without server": {
			attrs:     map[string]string{"auth_method": "integrated"},
			wantError: "Missing server",
		},
		"unknown method": {
			attrs:     map[string]string{"server": "exch01.contoso.local", "auth_method": "kerberos"},
			wantError: "Invalid auth_method",
		},
		"local Edge without server": {
			attrs:      map[string]string{"role": "Edge"},
			wantMethod: client.AuthPassword,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			for _, k := range authEnv {
				t.Setenv(k, "")
			}
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			cfg, diags := resolveConfig(model(c.attrs))

			var messages []string
			for _, d := range diags {
				messages = append(messages, d.Summary()+": "+d.Detail())
			}
			all := strings.Join(messages, "\n")
			if strings.Contains(all, secretPassword) {
				t.Errorf("a diagnostic leaks the password:\n%s", all)
			}
			switch {
			case c.wantError != "" && !strings.Contains(all, c.wantError):
				t.Fatalf("want an error containing %q, got:\n%s", c.wantError, all)
			case c.wantError == "" && diags.HasError():
				t.Fatalf("unexpected error:\n%s", all)
			case c.wantError != "":
				return
			}
			if cfg.AuthMethod != c.wantMethod {
				t.Errorf("AuthMethod = %q, want %q", cfg.AuthMethod, c.wantMethod)
			}
			if cfg.AuthMethod == client.AuthIntegrated && (cfg.Username != "" || cfg.Password != "") {
				t.Errorf("integrated configuration carries credentials: %q / %q", cfg.Username, cfg.Password)
			}
		})
	}
}

// The "unknown method" message must name the valid values.
func TestInvalidAuthMethodListsValues(t *testing.T) {
	for _, k := range authEnv {
		t.Setenv(k, "")
	}
	_, diags := resolveConfig(model(map[string]string{"server": "exch01.contoso.local", "auth_method": "kerberos"}))
	if len(diags) == 0 || !strings.Contains(diags[0].Detail(), "password, integrated") {
		t.Errorf("diagnostics = %v", diags)
	}
}
