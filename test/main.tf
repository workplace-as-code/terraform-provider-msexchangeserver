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
  comment = local.updated ? "[TF-CI-TEST] updated" : "[TF-CI-TEST] created"
}

# ---------------------------------------------------------------------------
# Data sources
# ---------------------------------------------------------------------------

data "msexchangeserver_accepted_domain" "default" {
  name = var.default_domain
}

data "msexchangeserver_accepted_domains" "all" {}

data "msexchangeserver_organization_config" "org" {}

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
