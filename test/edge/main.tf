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

output "accepted_domains" {
  description = "Accepted domains copied to the Edge server by EdgeSync."
  value       = [for d in data.msexchangeserver_accepted_domains.all.domains : d.domain_name]
}
