# Terraform Provider for Microsoft Exchange Server

Manage an on-premises Exchange Server organization as code with Terraform: mail flow rules, accepted and remote domains, Outlook on the web and mobile device policies, email address policies, retention and organization settings.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Terraform Registry](https://img.shields.io/badge/registry-workplace--as--code%2Fmsexchangeserver-7B42BC?logo=terraform)](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver)

- Review every change with `terraform plan` before it reaches Exchange
- Detect configuration drift made in the Exchange admin center or the Exchange Management Shell
- Adopt existing objects safely: attributes left out of the configuration are never written
- Start from an existing organization: `terraform query` finds its objects and generates their configuration
- Keep a full audit trail in version control

The provider is **experimental**: see [Resources and availability](docs/guides/availability.md).

---

## Quick Start

Terraform runs on a Windows machine joined to the Active Directory domain of the Exchange organization. The provider opens an Exchange remote PowerShell session with Kerberos.

```hcl
terraform {
  required_providers {
    msexchangeserver = {
      source  = "workplace-as-code/msexchangeserver"
      version = "~> 0.1"
    }
  }
}

provider "msexchangeserver" {
  server   = "exch01.contoso.local" # FQDN registered in Active Directory
  username = var.username           # CONTOSO\svc-terraform
  password = var.password
}

# Never auto-forward mail to a partner domain
resource "msexchangeserver_remote_domain" "partner" {
  name                 = "Partner"
  domain_name          = "partner.example"
  auto_forward_enabled = false
}
```

---

## Supported Resources

| Category | Resources |
|---|---|
| Mail flow | `msexchangeserver_transport_rule`, `msexchangeserver_accepted_domain`, `msexchangeserver_remote_domain`, `msexchangeserver_transport_config` |
| Clients and devices | `msexchangeserver_owa_mailbox_policy`, `msexchangeserver_mobile_device_mailbox_policy` |
| Organization | `msexchangeserver_organization_config`, `msexchangeserver_email_address_policy`, `msexchangeserver_retention_policy`, `msexchangeserver_retention_policy_tag` |

Data sources: `msexchangeserver_accepted_domain`, `msexchangeserver_accepted_domains`, `msexchangeserver_mailbox`, `msexchangeserver_organization_config`.

### Related providers

- [workplace-as-code/msexchangeonline](https://github.com/workplace-as-code/terraform-provider-msexchangeonline): Exchange Online, built on the same PowerShell bridge.
- [workplace-as-code/msteams](https://github.com/workplace-as-code/terraform-provider-msteams): Microsoft Teams policies and tenant settings.

---

## Installation

### Requirements
- Terraform >= 1.5, on a Windows machine joined to the Active Directory domain, with Windows PowerShell 5.1 (built into Windows)
- The FQDN of an Exchange server, and an account with the **Organization Management** role group or equivalent (see the [Authentication guide](docs/guides/authentication.md))

The PowerShell bridge scripts are embedded in the provider binary: nothing else needs to be installed. The provider keeps a few PowerShell processes open during a run, so the remote session is opened once per process instead of once per resource (see [Performance tuning](#performance-tuning)).

### Performance tuning

| Variable | Default | Effect |
|---|---|---|
| `EXCHANGE_MAX_CONCURRENCY` | `4` | Maximum number of PowerShell processes running at once |
| `EXCHANGE_SCRIPT_TIMEOUT` | `10m` | Maximum duration of one bridge call |
| `EXCHANGE_BRIDGE_MODE` | `persistent` | `oneshot` starts a new PowerShell process for every call |
| `EXCHANGE_WORKER_MAX_REQUESTS` | `200` | Requests served by a PowerShell process before it is replaced |
| `EXCHANGE_BRIDGE_DIR` | unset | Run the bridge scripts from this folder instead of the embedded copy (script development only) |

---

## Documentation

- [Getting Started](docs/guides/getting_started.md)
- [Authentication](docs/guides/authentication.md)
- [Import an existing organization](docs/guides/import_existing.md)
- [Detect drift in CI](docs/guides/drift_detection.md)
- [Resources and availability](docs/guides/availability.md)
- [Troubleshooting](docs/guides/troubleshooting.md)
- [All Resources](docs/resources/)
- [All Data Sources](docs/data-sources/)

---

## Contributing

Contributions are welcome. Read the [contributing guide](CONTRIBUTING.md), then open an issue for a [bug](.github/ISSUE_TEMPLATE/bug_report.md), a [feature](.github/ISSUE_TEMPLATE/feature_request.md) or a [new resource](.github/ISSUE_TEMPLATE/new_resource.md).

## License

[MIT](LICENSE)
