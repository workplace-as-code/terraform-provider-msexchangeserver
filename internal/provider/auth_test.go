package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// authEnv lists every variable resolveConfig reads. Each case starts with all
// of them empty, so the shell running the tests cannot change the result.
var authEnv = []string{"EXCHANGE_SERVER", "EXCHANGE_SERVERS", "EXCHANGE_USERNAME", "EXCHANGE_PASSWORD", "EXCHANGE_ROLE", "EXCHANGE_AUTH_METHOD", "EXCHANGE_DOMAIN_CONTROLLER"}

// secretPassword must never appear in a diagnostic.
const secretPassword = "pw-secret"

// model builds a provider block from attribute values; absent keys are null.
// "servers" is a comma-separated list.
func model(attrs map[string]string) exchangeProviderModel {
	v := func(name string) types.String {
		if s, ok := attrs[name]; ok {
			return types.StringValue(s)
		}
		return types.StringNull()
	}
	servers := types.ListNull(types.StringType)
	if list, ok := attrs["servers"]; ok {
		var elems []attr.Value
		for _, s := range strings.Split(list, ",") {
			elems = append(elems, types.StringValue(s))
		}
		servers = types.ListValueMust(types.StringType, elems)
	}
	return exchangeProviderModel{
		Server:     v("server"),
		Servers:    servers,
		Username:   v("username"),
		Password:   v("password"),
		Role:       v("role"),
		AuthMethod: v("auth_method"),

		DomainController: v("domain_controller"),
	}
}

func TestResolveConfig(t *testing.T) {
	server := map[string]string{"server": "exch01.contoso.local"}
	withUser := map[string]string{"server": "exch01.contoso.local", "username": "CONTOSO\\svc", "password": secretPassword}
	integrated := map[string]string{"server": "exch01.contoso.local", "auth_method": "integrated"}

	cases := map[string]struct {
		attrs       map[string]string
		env         map[string]string
		wantError   string // substring of a diagnostic summary, "" when the configuration is valid
		wantMethod  string
		wantDC      string
		wantServers string // comma-separated
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
		"domain controller": {
			attrs:      map[string]string{"server": "exch01.contoso.local", "auth_method": "integrated", "domain_controller": "dc01.contoso.local"},
			wantMethod: client.AuthIntegrated,
			wantDC:     "dc01.contoso.local",
		},
		"domain controller from the environment": {
			attrs:      integrated,
			env:        map[string]string{"EXCHANGE_DOMAIN_CONTROLLER": " dc02.contoso.local "},
			wantMethod: client.AuthIntegrated,
			wantDC:     "dc02.contoso.local",
		},
		"domain controller on Edge": {
			attrs:     map[string]string{"role": "Edge", "domain_controller": "dc01.contoso.local"},
			wantError: "domain_controller not available on Edge",
		},
		"servers": {
			attrs:       map[string]string{"servers": "exch01.contoso.local, exch02.contoso.local", "auth_method": "integrated"},
			wantMethod:  client.AuthIntegrated,
			wantServers: "exch01.contoso.local,exch02.contoso.local",
		},
		"servers from the environment": {
			attrs:       map[string]string{"auth_method": "integrated"},
			env:         map[string]string{"EXCHANGE_SERVERS": "exch01.contoso.local,,exch02.contoso.local"},
			wantMethod:  client.AuthIntegrated,
			wantServers: "exch01.contoso.local,exch02.contoso.local",
		},
		"servers in the block ignore EXCHANGE_SERVER": {
			attrs:       map[string]string{"servers": "exch01.contoso.local", "auth_method": "integrated"},
			env:         map[string]string{"EXCHANGE_SERVER": "exch09.contoso.local"},
			wantMethod:  client.AuthIntegrated,
			wantServers: "exch01.contoso.local",
		},
		"server and servers": {
			attrs:     map[string]string{"server": "exch01.contoso.local", "servers": "exch02.contoso.local", "auth_method": "integrated"},
			wantError: "Conflicting server settings",
		},
		"EXCHANGE_SERVER and EXCHANGE_SERVERS": {
			attrs:     map[string]string{"auth_method": "integrated"},
			env:       map[string]string{"EXCHANGE_SERVER": "exch01.contoso.local", "EXCHANGE_SERVERS": "exch02.contoso.local"},
			wantError: "Conflicting server settings",
		},
		"servers on Edge": {
			attrs:     map[string]string{"role": "Edge", "servers": "edge01,edge02", "username": "EDGE01\\Administrator"},
			wantError: "servers not available on Edge",
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
			if cfg.DomainController != c.wantDC {
				t.Errorf("DomainController = %q, want %q", cfg.DomainController, c.wantDC)
			}
			if got := strings.Join(cfg.Servers, ","); got != c.wantServers {
				t.Errorf("Servers = %q, want %q", got, c.wantServers)
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
