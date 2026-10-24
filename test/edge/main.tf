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

output "accepted_domains" {
  description = "Accepted domains copied to the Edge server by EdgeSync."
  value       = [for d in data.msexchangeserver_accepted_domains.all.domains : d.domain_name]
}
