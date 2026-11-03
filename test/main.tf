terraform {
  required_providers {
    msexchangeserver = {
      source = "workplace-as-code/msexchangeserver"
    }
  }
}

# Connection settings come from the EXCHANGE_* environment variables set by
# test/run.ps1: EXCHANGE_SERVER, EXCHANGE_USERNAME and EXCHANGE_PASSWORD.
provider "msexchangeserver" {}

variable "default_domain" {
  description = "An existing accepted domain of the organization, read by the data source. Set by run.ps1."
  type        = string
}

variable "exchange_server" {
  description = "The Exchange server of the test, whose virtual directories are adopted. Set by run.ps1."
  type        = string
}

variable "test_database" {
  description = "A mailbox database of the tested server, adopted by the test. Set by run.ps1."
  type        = string
}

variable "test_database_retention_days" {
  description = "Deleted item retention of test_database before the test, restored at the end. Set by run.ps1."
  type        = number
}

variable "test_remote_mailbox" {
  description = "Identity of an existing remote mailbox, read by the data source. Set by run.ps1; empty skips it."
  type        = string
  default     = ""
}

variable "phase" {
  description = "Set to \"update\" by run.ps1 to change one or more attributes of every resource in place."
  type        = string
  default     = "create"

  validation {
    condition     = contains(["create", "update"], var.phase)
    error_message = "phase must be \"create\" or \"update\"."
  }
}

locals {
  updated = var.phase == "update"
  # Short name of the tested server, as Exchange shows it in identities.
  server_name = upper(split(".", var.exchange_server)[0])
  comment     = local.updated ? "[TF-CI-TEST] updated" : "[TF-CI-TEST] created"
}

# ---------------------------------------------------------------------------
# Data sources
# ---------------------------------------------------------------------------

data "msexchangeserver_accepted_domain" "default" {
  name = var.default_domain
}

data "msexchangeserver_accepted_domains" "all" {}

data "msexchangeserver_organization_config" "org" {}

data "msexchangeserver_remote_mailbox" "test" {
  count    = var.test_remote_mailbox != "" ? 1 : 0
  identity = var.test_remote_mailbox

  lifecycle {
    postcondition {
      condition     = self.recipient_type_details != "" && self.remote_routing_address != ""
      error_message = "The remote mailbox was read without its type or remote routing address."
    }
  }
}

# ---------------------------------------------------------------------------
# Organization-wide objects: imported, never modified. run.ps1 removes them
# from state before the destroy.
# ---------------------------------------------------------------------------

import {
  to = msexchangeserver_organization_config.this
  id = "organization"
}

resource "msexchangeserver_organization_config" "this" {}

import {
  to = msexchangeserver_transport_config.this
  id = "default"
}

resource "msexchangeserver_transport_config" "this" {}

# ---------------------------------------------------------------------------
# Mail flow
# ---------------------------------------------------------------------------

# Exchange Server creates and removes accepted domains. tf-ci-test.example is
# a reserved name: no mail is ever delivered to it.
resource "msexchangeserver_accepted_domain" "test" {
  name        = "TF-CI-AcceptedDomain"
  domain_name = "tf-ci-test.example"
  domain_type = local.updated ? "InternalRelay" : "Authoritative"
}

resource "msexchangeserver_remote_domain" "test" {
  name        = "TF-CI-Domain"
  domain_name = "tf-ci-remote.example"

  auto_reply_enabled            = false
  auto_forward_enabled          = local.updated
  trusted_mail_inbound_enabled  = false
  trusted_mail_outbound_enabled = false
  allowed_oof_type              = "None"
}

# Disabled: the rule never acts on mail.
resource "msexchangeserver_transport_rule" "test" {
  name    = "TF-CI-Rule"
  enabled = false

  subject_contains = local.updated ? ["[TF-CI-TEST]", "[TF-CI-UPDATED]"] : ["[TF-CI-TEST]"]
  action           = "Reject"
  reject_message   = local.comment
}

# ---------------------------------------------------------------------------
# Clients and devices
# ---------------------------------------------------------------------------

resource "msexchangeserver_owa_mailbox_policy" "test" {
  name = "TF-CI-OWA"

  attachments_enabled     = true
  rules_enabled           = true
  signatures_enabled      = true
  theme_selection_enabled = local.updated
  blocked_file_types      = local.updated ? ["exe", "bat", "ps1", "vbs"] : ["exe", "bat", "ps1"]
}

resource "msexchangeserver_mobile_device_mailbox_policy" "test" {
  name                = "TF-CI-Mobile"
  password_enabled    = true
  min_password_length = local.updated ? 8 : 6
  allow_camera        = !local.updated
}

# ---------------------------------------------------------------------------
# Organization
# ---------------------------------------------------------------------------

# The filter matches no recipient (no mailbox has this department), so
# apply_to_recipients runs Update-EmailAddressPolicy without changing anyone.
resource "msexchangeserver_email_address_policy" "test" {
  name                            = "TF-CI-AddressPolicy"
  enabled_email_address_templates = ["SMTP:%m@${msexchangeserver_accepted_domain.test.domain_name}"]
  included_recipients             = "MailboxUsers"
  conditional_department          = local.updated ? "TF-CI-Updated" : "TF-CI-None"
  priority                        = 1
  apply_to_recipients             = true
}

resource "msexchangeserver_retention_policy_tag" "test" {
  name                    = "TF-CI-RetentionTag"
  type                    = "Personal"
  age_limit_for_retention = local.updated ? 60 : 30
  retention_action        = "DeleteAndAllowRecovery"
  comment                 = local.comment
}

resource "msexchangeserver_retention_policy" "test" {
  name                       = "TF-CI-Retention"
  retention_policy_tag_links = local.updated ? [msexchangeserver_retention_policy_tag.test.name] : []
}

# ---------------------------------------------------------------------------
# Outputs
# ---------------------------------------------------------------------------

output "default_domain" {
  description = "Domain name read by the accepted domain data source."
  value       = data.msexchangeserver_accepted_domain.default.domain_name
}

# The data source has no dependency on the test objects, so it may be read
# before or after tf-ci-test.example is created: leave the test domain out, or
# the drift check sees the output change.
output "accepted_domains" {
  description = "Accepted domains of the organization, without the test domain."
  value       = [for d in data.msexchangeserver_accepted_domains.all.domains : d.domain_name if !startswith(d.domain_name, "tf-ci-")]
}

# ---------------------------------------------------------------------------
# Per-server objects, adopted and never created or removed. Only the external
# URL of the OAB virtual directory changes (update phase); run.ps1 clears it
# again before the destroy.
# ---------------------------------------------------------------------------

resource "msexchangeserver_owa_virtual_directory" "test" {
  name = var.exchange_server
}

resource "msexchangeserver_ecp_virtual_directory" "test" {
  name = var.exchange_server
}

resource "msexchangeserver_web_services_virtual_directory" "test" {
  name = var.exchange_server
}

resource "msexchangeserver_activesync_virtual_directory" "test" {
  name = var.exchange_server
}

resource "msexchangeserver_oab_virtual_directory" "test" {
  name         = var.exchange_server
  external_url = local.updated ? "https://tf-ci.invalid/OAB" : ""
}

resource "msexchangeserver_mapi_virtual_directory" "test" {
  name = var.exchange_server
}

resource "msexchangeserver_client_access_service" "test" {
  name = var.exchange_server
}

# The update phase changes the deleted item retention by one day; run.ps1 puts
# the original value back before the destroy.
resource "msexchangeserver_mailbox_database" "test" {
  name = var.test_database
  deleted_item_retention_days = local.updated ? (
    var.test_database_retention_days >= 30 ? var.test_database_retention_days - 1 : var.test_database_retention_days + 1
  ) : var.test_database_retention_days
}

# ---------------------------------------------------------------------------
# Connectors: disabled, with documentation addresses and domains, so that
# they never send or accept mail.
# ---------------------------------------------------------------------------

resource "msexchangeserver_send_connector" "test" {
  name                     = "TF-CI-Send"
  usage                    = "Custom"
  address_spaces           = local.updated ? ["tf-ci-test.example", "tf-ci-partner.example;10"] : ["tf-ci-test.example"]
  smart_hosts              = ["smtp.tf-ci-test.example"]
  dns_routing_enabled      = false
  source_transport_servers = [local.server_name]
  max_message_size_mb      = 10
  require_tls              = local.updated
  tls_auth_level           = local.updated ? "EncryptionOnly" : null
  enabled                  = false
}

resource "msexchangeserver_receive_connector" "test" {
  name              = "${local.server_name}\\TF-CI-Receive"
  usage             = "Custom"
  transport_role    = "FrontendTransport"
  bindings          = ["0.0.0.0:2526"]
  remote_ip_ranges  = local.updated ? ["192.0.2.10", "198.51.100.0/24"] : ["192.0.2.10"]
  permission_groups = ["ExchangeServers"]
  auth_mechanism    = ["Tls", "ExternalAuthoritative"]
  banner            = local.updated ? "220 TF-CI updated" : null
  enabled           = false
}
