# Changelog

## 0.1.0 (October 24, 2026)

First release, experimental.

### Resources

- Mail flow: `msexchangeserver_transport_rule`, `msexchangeserver_accepted_domain`, `msexchangeserver_remote_domain`, `msexchangeserver_transport_config`.
- Clients and devices: `msexchangeserver_owa_mailbox_policy`, `msexchangeserver_mobile_device_mailbox_policy`.
- Organization: `msexchangeserver_organization_config`, `msexchangeserver_email_address_policy`, `msexchangeserver_retention_policy`, `msexchangeserver_retention_policy_tag`.

### Data sources

- `msexchangeserver_accepted_domain`, `msexchangeserver_accepted_domains`, `msexchangeserver_mailbox`, `msexchangeserver_organization_config`.

### Provider

- Exchange remote PowerShell with Kerberos authentication, from a Windows machine joined to the domain, with Windows PowerShell 5.1.
- Existing configuration can be imported in bulk: every resource has a list resource for `terraform query` (Terraform 1.14+), which finds the existing objects and generates their configuration with `-generate-config-out`. Resources also have a resource identity, so `import` blocks accept `identity = { id = "..." }`.
- Every resource only manages the attributes set in the configuration: an attribute left out is never written, so importing or adopting an existing object never resets one of its settings to a value chosen by the provider. New objects get the Exchange defaults.
- `msexchangeserver_email_address_policy` only applies a policy to the existing recipients (`Update-EmailAddressPolicy`) when `apply_to_recipients` is `true`, after its creation and after a change of its settings.
- Destroying an organization-wide setting shows a warning: it is only removed from the Terraform state and keeps its current values.
- The PowerShell scripts are embedded in the provider binary. PowerShell sessions are kept open during a run, with a cap on concurrent sessions and a timeout per call (`EXCHANGE_MAX_CONCURRENCY`, `EXCHANGE_SCRIPT_TIMEOUT`, ...).
- Edge Transport servers: `role = "Edge"` connects to an Edge server, locally or through WinRM with a local account, and limits the plan to the resources that exist there (transport rules, with the `Quarantine` action, accepted and remote domains, transport settings). On Edge, `Reject` rejects the message during the SMTP session, and `attachment_is_executable` is refused at plan time.
- Clear advice for the common connection errors: Kerberos (machine outside the domain, IP address instead of the FQDN), access denied, and cmdlets missing from the account's management roles.
