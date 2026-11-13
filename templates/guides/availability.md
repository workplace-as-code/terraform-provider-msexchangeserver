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
| [`msexchangeserver_send_connector`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/send_connector) | Not on Edge (EdgeSync manages them) |
| [`msexchangeserver_receive_connector`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/receive_connector) | Mailbox and Edge |
| [`msexchangeserver_journal_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/journal_rule) | Needs Exchange Enterprise client access licenses |

## Clients, recipients and organization

| Resource | Notes |
|---|---|
| [`msexchangeserver_organization_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/organization_config) | Singleton |
| [`msexchangeserver_owa_mailbox_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/owa_mailbox_policy) | Outlook on the web |
| [`msexchangeserver_mobile_device_mailbox_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/mobile_device_mailbox_policy) | Exchange ActiveSync devices |
| [`msexchangeserver_activesync_organization_settings`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/activesync_organization_settings) | Singleton: access of the devices that match no rule |
| [`msexchangeserver_activesync_device_access_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/activesync_device_access_rule) | Allow, block or quarantine by device type, model, OS or user agent |
| [`msexchangeserver_email_address_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/email_address_policy) | Every update applies the policy to the matching recipients (`Update-EmailAddressPolicy`) |
| [`msexchangeserver_retention_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/retention_policy) | Messaging records management |
| [`msexchangeserver_retention_policy_tag`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/retention_policy_tag) | |
| [`msexchangeserver_admin_audit_log_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/admin_audit_log_config) | Singleton: administrator audit logging |
| [`msexchangeserver_authentication_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/authentication_policy) | Exchange 2019 CU2 and later; the organization default is set on `msexchangeserver_organization_config` |

## Servers

Objects that exist once per server, and mailbox databases: the resources adopt them by server or database name and never create or remove them. Certificates are the exception: the provider imports and removes them.

| Resource | Notes |
|---|---|
| [`msexchangeserver_owa_virtual_directory`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/owa_virtual_directory) | URLs and authentication (IIS); Extended Protection read only |
| [`msexchangeserver_ecp_virtual_directory`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/ecp_virtual_directory) | URLs and authentication (IIS); Extended Protection read only |
| [`msexchangeserver_web_services_virtual_directory`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/web_services_virtual_directory) | URLs and authentication (IIS); Extended Protection read only |
| [`msexchangeserver_activesync_virtual_directory`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/activesync_virtual_directory) | URLs and authentication (IIS); Extended Protection read only |
| [`msexchangeserver_oab_virtual_directory`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/oab_virtual_directory) | URLs and authentication (IIS); Extended Protection read only |
| [`msexchangeserver_mapi_virtual_directory`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/mapi_virtual_directory) | URLs and IIS authentication methods; Extended Protection read only |
| [`msexchangeserver_outlook_anywhere`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/outlook_anywhere) | Host names, authentication and SSL of Outlook Anywhere (`/Rpc`) |
| [`msexchangeserver_client_access_service`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/client_access_service) | Autodiscover URL in Active Directory |
| [`msexchangeserver_mailbox_database`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/mailbox_database) | Quotas, retention and logging; never mounted, dismounted or removed |
| [`msexchangeserver_transport_service`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/transport_service) | Message tracking, connectivity and protocol logs; Mailbox and Edge |
| [`msexchangeserver_frontend_transport_service`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/frontend_transport_service) | Connectivity and protocol logs of the Front End Transport service |
| [`msexchangeserver_pop_settings`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/pop_settings) | Sign-in, certificate, bindings and limits; the services are not restarted |
| [`msexchangeserver_imap_settings`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/imap_settings) | Sign-in, certificate, bindings and limits; the services are not restarted |
| [`msexchangeserver_exchange_certificate`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/exchange_certificate) | Imported from a PFX (write-only, Terraform 1.11 and later) and removed on destroy |
| [`msexchangeserver_exchange_certificate_services`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/exchange_certificate_services) | IIS, SMTP, IMAP and POP; only ever added |

## Edge Transport

Anti-spam agents and address rewriting of Edge Transport servers (`role = "Edge"`).

| Resource | Notes |
|---|---|
| [`msexchangeserver_content_filter_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/content_filter_config) | Singleton |
| [`msexchangeserver_sender_filter_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/sender_filter_config) | Singleton |
| [`msexchangeserver_recipient_filter_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/recipient_filter_config) | Singleton |
| [`msexchangeserver_ip_allow_list_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/ip_allow_list_config) | Singleton |
| [`msexchangeserver_ip_block_list_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/ip_block_list_config) | Singleton |
| [`msexchangeserver_ip_block_list_providers_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/ip_block_list_providers_config) | Singleton |
| [`msexchangeserver_ip_allow_list_entry`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/ip_allow_list_entry) |  |
| [`msexchangeserver_ip_block_list_entry`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/ip_block_list_entry) |  |
| [`msexchangeserver_ip_block_list_provider`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/ip_block_list_provider) |  |
| [`msexchangeserver_address_rewrite_entry`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/address_rewrite_entry) |  |

## Data sources

Every resource has a data source of the same name, which reads an object without managing it, except `msexchangeserver_activesync_device_access_rule`, `msexchangeserver_activesync_organization_settings`, `msexchangeserver_admin_audit_log_config`, `msexchangeserver_content_filter_config`, `msexchangeserver_exchange_certificate_services`, `msexchangeserver_ip_allow_list_config`, `msexchangeserver_ip_block_list_config`, `msexchangeserver_ip_block_list_providers_config`, `msexchangeserver_recipient_filter_config`, `msexchangeserver_sender_filter_config` and `msexchangeserver_transport_config`.

These data sources have no resource:

| Data source | Notes |
|---|---|
| [`msexchangeserver_accepted_domains`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/data-sources/accepted_domains) | Every accepted domain of the organization |
| [`msexchangeserver_mailbox`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/data-sources/mailbox) | One mailbox, read only |
| [`msexchangeserver_remote_mailbox`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/data-sources/remote_mailbox) | Hybrid: the on-premises object of a mailbox hosted in Exchange Online, read only |

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
