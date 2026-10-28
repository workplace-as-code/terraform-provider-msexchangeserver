package provider

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// resolveConfig turns the provider block and the EXCHANGE_* variables into the
// client configuration, or explains what is wrong with them. It does not
// contact Exchange: the connection opens on the first operation.
func resolveConfig(m exchangeProviderModel) (client.Config, diag.Diagnostics) {
	var diags diag.Diagnostics

	server, servers, conflict := resolveServers(m)
	if conflict != "" {
		diags.AddError("Conflicting server settings",
			fmt.Sprintf("Set either server or servers, not both (%s).", conflict))
		return client.Config{}, diags
	}

	cfg := client.Config{
		Server:     server,
		Servers:    servers,
		Username:   envOrConfig("EXCHANGE_USERNAME", m.Username),
		Password:   envOrConfig("EXCHANGE_PASSWORD", m.Password),
		Role:       normalizeRole(envOrConfig("EXCHANGE_ROLE", m.Role)),
		AuthMethod: normalizeAuthMethod(envOrConfig("EXCHANGE_AUTH_METHOD", m.AuthMethod)),

		DomainController: strings.TrimSpace(envOrConfig("EXCHANGE_DOMAIN_CONTROLLER", m.DomainController)),
	}
	if cfg.Role == "" {
		diags.AddError("Invalid role", "role must be \"Mailbox\" or \"Edge\". Check the provider block and EXCHANGE_ROLE.")
		return cfg, diags
	}
	if cfg.AuthMethod == "" {
		diags.AddError("Invalid auth_method",
			fmt.Sprintf("auth_method must be one of: %s. Check the provider block and EXCHANGE_AUTH_METHOD.", strings.Join([]string{client.AuthPassword, client.AuthIntegrated}, ", ")))
		return cfg, diags
	}

	if cfg.AuthMethod == client.AuthIntegrated {
		// A username or password left in the block or the environment must not
		// be silently ignored: the user may believe that account connects.
		var set []string
		if cfg.Username != "" {
			set = append(set, "username")
		}
		if cfg.Password != "" {
			set = append(set, "password")
		}
		if len(set) > 0 {
			diags.AddError("Conflicting authentication settings",
				fmt.Sprintf("auth_method = \"integrated\" connects with the Windows identity of the process running Terraform, but %s is set (in the provider block or an EXCHANGE_* variable): choose one method.", strings.Join(set, " and ")))
		}
		if cfg.Role == client.RoleEdge {
			diags.AddError("Integrated authentication not available on Edge",
				"auth_method = \"integrated\" uses Kerberos, which an Edge Transport server outside the domain does not accept. To use the current Windows account on Edge, run Terraform on the Edge server and leave server out.")
		}
		cfg.Username, cfg.Password = "", ""
	}

	// Edge Transport servers keep their configuration in AD LDS, not in
	// Active Directory: there is no domain controller to choose.
	if cfg.DomainController != "" && cfg.Role == client.RoleEdge {
		diags.AddError("domain_controller not available on Edge",
			"domain_controller only applies to the Mailbox role. Remove it from the provider block or EXCHANGE_DOMAIN_CONTROLLER.")
	}
	// An Edge Transport server stands alone: there is no other server to try.
	if len(cfg.Servers) > 0 && cfg.Role == client.RoleEdge {
		diags.AddError("servers not available on Edge",
			"servers only applies to the Mailbox role. Use server with the Edge Transport server, or leave it out when Terraform runs on it.")
	}

	// A local Edge Transport server needs no connection settings: Terraform
	// runs on it with the current Windows account.
	connects := cfg.Server != "" || len(cfg.Servers) > 0
	if !connects && cfg.Role == client.RoleMailbox {
		diags.AddError("Missing server", "server or servers is required. Set it in the provider block or via EXCHANGE_SERVER or EXCHANGE_SERVERS.")
	}
	if cfg.AuthMethod == client.AuthPassword && cfg.Username == "" && connects {
		diags.AddError("Missing username", "username is required. Set it in the provider block or via EXCHANGE_USERNAME.")
	}
	return cfg, diags
}

// resolveServers returns server or servers, whichever is set, and describes
// the conflict when both are. As with the other settings, the provider block
// wins over the environment: a server in the block ignores EXCHANGE_SERVERS,
// and servers in the block ignore EXCHANGE_SERVER.
func resolveServers(m exchangeProviderModel) (string, []string, string) {
	server := ""
	if !m.Server.IsNull() && !m.Server.IsUnknown() {
		server = strings.TrimSpace(m.Server.ValueString())
	}
	var servers []string
	if !m.Servers.IsNull() && !m.Servers.IsUnknown() {
		for _, e := range m.Servers.Elements() {
			if s, ok := e.(types.String); ok && !s.IsNull() && !s.IsUnknown() {
				servers = appendServers(servers, s.ValueString())
			}
		}
	}
	switch {
	case server != "" && len(servers) > 0:
		return "", nil, "both are in the provider block"
	case server != "" || len(servers) > 0:
		return server, servers, ""
	}

	server = strings.TrimSpace(os.Getenv("EXCHANGE_SERVER"))
	servers = appendServers(nil, os.Getenv("EXCHANGE_SERVERS"))
	if server != "" && len(servers) > 0 {
		return "", nil, "EXCHANGE_SERVER and EXCHANGE_SERVERS are both set"
	}
	return server, servers, ""
}

// appendServers appends the non-empty names of a comma-separated list.
func appendServers(servers []string, list string) []string {
	for _, s := range strings.Split(list, ",") {
		if s = strings.TrimSpace(s); s != "" {
			servers = append(servers, s)
		}
	}
	return servers
}

// normalizeAuthMethod returns client.AuthPassword (also for an empty value) or
// client.AuthIntegrated, whatever the case, or "" for an unknown method.
func normalizeAuthMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", client.AuthPassword:
		return client.AuthPassword
	case client.AuthIntegrated:
		return client.AuthIntegrated
	}
	return ""
}
