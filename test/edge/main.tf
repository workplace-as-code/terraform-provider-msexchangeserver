terraform {
  required_providers {
    msexchangeserver = {
      source = "workplace-as-code/msexchangeserver"
    }
  }
}

# Connection settings come from the EXCHANGE_* environment variables set by
# test/edge/run.ps1: EXCHANGE_ROLE=Edge, plus EXCHANGE_SERVER,
# EXCHANGE_USERNAME and EXCHANGE_PASSWORD for a remote Edge server.
provider "msexchangeserver" {}

variable "edge_server" {
  description = "Short name of the Edge server, as Exchange shows it in identities. Set by run.ps1."
  type        = string
}

variable "phase" {
  description = "Set to \"update\" by run.ps1 to change the test rule in place."
  type        = string
  default     = "create"

  validation {
    condition     = contains(["create", "update"], var.phase)
    error_message = "phase must be \"create\" or \"update\"."
  }
}

variable "connectivity_log_max_age_days" {
  description = "Set by run.ps1: the original value plus one for the update, the original to restore. null leaves it unmanaged."
  type        = number
  default     = null
}

locals {
  updated = var.phase == "update"
}

# Accepted domains are copied to the Edge server by EdgeSync: read only.
data "msexchangeserver_accepted_domains" "all" {}

# Imported, never modified. run.ps1 removes it from state before the destroy.
import {
  to = msexchangeserver_transport_config.this
  id = "default"
}

resource "msexchangeserver_transport_config" "this" {}

# The Transport service of the Edge server, adopted: run.ps1 raises its
# connectivity log age by one day for the update and restores it.
resource "msexchangeserver_transport_service" "this" {
  name                          = var.edge_server
  connectivity_log_max_age_days = var.connectivity_log_max_age_days
}

# Edge transport rules are local to the Edge server (EdgeSync does not copy
# them). Quarantine only exists on Edge; the update switches to Reject, which
# clears Quarantine. Disabled: the rule never acts on mail.
resource "msexchangeserver_transport_rule" "test" {
  name    = "TF-CI-EdgeRule"
  enabled = false

  subject_contains = local.updated ? ["[TF-CI-TEST]", "[TF-CI-UPDATED]"] : ["[TF-CI-TEST]"]
  action           = local.updated ? "Reject" : "Quarantine"
  reject_message   = local.updated ? "[TF-CI-TEST] updated" : null
}

# Receive connectors are local to the Edge server. Disabled, on an unused port
# and for a documentation range: it never accepts mail.
resource "msexchangeserver_receive_connector" "test" {
  name             = "${var.edge_server}\\TF-CI-EdgeReceive"
  bindings         = ["0.0.0.0:2526"]
  remote_ip_ranges = local.updated ? ["192.0.2.10", "198.51.100.0/24"] : ["192.0.2.10"]
  banner           = local.updated ? "220 TF-CI updated" : null
  enabled          = false
}

# Anti-spam settings of the Edge server, imported. The update phase changes
# three of them; run.ps1 puts back these values, the Exchange defaults, before
# the destroy. The others are only read.
import {
  to = msexchangeserver_content_filter_config.this
  id = "default"
}

resource "msexchangeserver_content_filter_config" "this" {
  scl_reject_threshold = local.updated ? 6 : 7
}

import {
  to = msexchangeserver_sender_filter_config.this
  id = "default"
}

resource "msexchangeserver_sender_filter_config" "this" {
  blank_sender_blocking_enabled = local.updated
}

import {
  to = msexchangeserver_recipient_filter_config.this
  id = "default"
}

resource "msexchangeserver_recipient_filter_config" "this" {
  blocked_recipients = local.updated ? ["tf-ci-blocked@tf-ci-test.example"] : []
}

import {
  to = msexchangeserver_ip_allow_list_config.this
  id = "default"
}

resource "msexchangeserver_ip_allow_list_config" "this" {}

import {
  to = msexchangeserver_ip_block_list_config.this
  id = "default"
}

resource "msexchangeserver_ip_block_list_config" "this" {}

import {
  to = msexchangeserver_ip_block_list_providers_config.this
  id = "default"
}

resource "msexchangeserver_ip_block_list_providers_config" "this" {}

# Test objects with documentation addresses and domains. An IP list entry has
# no Set- cmdlet: the new comment of the update phase replaces it.
resource "msexchangeserver_ip_allow_list_entry" "test" {
  name    = "192.0.2.10"
  comment = local.updated ? "[TF-CI-TEST] updated" : "[TF-CI-TEST]"
}

resource "msexchangeserver_ip_block_list_entry" "test" {
  name            = "198.51.100.0/24"
  comment         = "[TF-CI-TEST]"
  expiration_time = "2030-01-01T00:00:00Z"
}

resource "msexchangeserver_ip_block_list_provider" "test" {
  name               = "TF-CI-DNSBL"
  lookup_domain      = "dnsbl.tf-ci-test.example"
  enabled            = false
  rejection_response = local.updated ? "[TF-CI-TEST] updated" : "[TF-CI-TEST] rejected"
}

resource "msexchangeserver_address_rewrite_entry" "test" {
  name             = "TF-CI-Rewrite"
  internal_address = "tf-ci-internal.example"
  external_address = "tf-ci-test.example"
  outbound_only    = local.updated
}

output "accepted_domains" {
  description = "Accepted domains copied to the Edge server by EdgeSync."
  value       = [for d in data.msexchangeserver_accepted_domains.all.domains : d.domain_name]
}
