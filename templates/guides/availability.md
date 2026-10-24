---
page_title: "Resources and Availability"
subcategory: "Guides"
---

# Resources and Availability

The provider is **experimental**. It is tested against Exchange Server Subscription Edition in a lab with `test/run.ps1`, which creates, updates, imports and destroys every resource below. Report any difference with your Exchange version in an issue.

Edge Transport servers (`role = "Edge"`) are tested with `test/edge/run.ps1`, locally and through WinRM, on an Edge server subscribed to EdgeSync: a transport rule is created with the `Quarantine` action, updated to `Reject`, imported and destroyed, and the accepted domains, remote domains and transport settings are imported and listed with `terraform query`. Accepted and remote domains are only read on Edge, since EdgeSync manages them.

Exchange Online is managed by a separate provider, [workplace-as-code/msexchangeonline](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest).

See the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/authentication) for the requirements of the machine running Terraform.

## Mail flow

| Resource | Notes |
|---|---|
| [`msexchangeserver_transport_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/transport_rule) | Mail flow rules |
| [`msexchangeserver_accepted_domain`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/accepted_domain) | `Authoritative`, `InternalRelay` or `ExternalRelay` |
| [`msexchangeserver_remote_domain`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/remote_domain) | Auto-reply, auto-forward and formatting per external domain |
| [`msexchangeserver_transport_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/transport_config) | Singleton: organization-wide transport settings |

## Clients, recipients and organization

| Resource | Notes |
|---|---|
| [`msexchangeserver_organization_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/organization_config) | Singleton |
| [`msexchangeserver_owa_mailbox_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/owa_mailbox_policy) | Outlook on the web |
| [`msexchangeserver_mobile_device_mailbox_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/mobile_device_mailbox_policy) | Exchange ActiveSync devices |
| [`msexchangeserver_email_address_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/email_address_policy) | Every update applies the policy to the matching recipients (`Update-EmailAddressPolicy`) |
| [`msexchangeserver_retention_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/retention_policy) | Messaging records management |
| [`msexchangeserver_retention_policy_tag`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/retention_policy_tag) | |

## Data sources

| Data source |
|---|
| [`msexchangeserver_accepted_domain`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/data-sources/accepted_domain) |
| [`msexchangeserver_accepted_domains`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/data-sources/accepted_domains) |
| [`msexchangeserver_mailbox`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/data-sources/mailbox) |
| [`msexchangeserver_organization_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/data-sources/organization_config) |

## Singletons

Organization-wide settings (`organization_config`, `transport_config`) always exist. Creating the resource adopts the current settings, and destroying it only removes it from Terraform state. Only the attributes set in the configuration are managed: the others keep their current value. Use an `import` block to start from the current settings:

`test/run.ps1` only imports them, so that the tests never change the organization. The writes of `transport_config` (sizes, Unlimited limits, durations, lists, enums) and its drift detection were tested by hand with `test/transport`, on a Mailbox server.

```terraform
import {
  to = msexchangeserver_transport_config.this
  id = "default"
}

resource "msexchangeserver_transport_config" "this" {
  external_postmaster_address = "postmaster@contoso.com"
}
```
