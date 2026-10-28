package provider

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/workplace-as-code/terraform-provider-msexchangeserver/internal/client"
)

// resolveConfig turns the provider block and the EXCHANGE_* variables into the
// client configuration, or explains what is wrong with them. It does not
// contact Exchange: the connection opens on the first operation.
func resolveConfig(m exchangeProviderModel) (client.Config, diag.Diagnostics) {
	var diags diag.Diagnostics

	cfg := client.Config{
		Server:     envOrConfig("EXCHANGE_SERVER", m.Server),
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

	// A local Edge Transport server needs no connection settings: Terraform
	// runs on it with the current Windows account.
	if cfg.Server == "" && cfg.Role == client.RoleMailbox {
		diags.AddError("Missing server", "server is required. Set it in the provider block or via EXCHANGE_SERVER.")
	}
	if cfg.AuthMethod == client.AuthPassword && cfg.Username == "" && cfg.Server != "" {
		diags.AddError("Missing username", "username is required. Set it in the provider block or via EXCHANGE_USERNAME.")
	}
	return cfg, diags
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
