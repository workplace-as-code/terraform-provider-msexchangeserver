terraform {
  required_providers {
    msexchangeserver = {
      source = "workplace-as-code/msexchangeserver"
    }
  }
}

# Manual test of the transport settings, which test/run.ps1 only imports.
# Connection settings come from the EXCHANGE_* variables set by env.ps1.
provider "msexchangeserver" {}

import {
  to = msexchangeserver_transport_config.this
  id = "default"
}

# Uncomment the attributes to test, apply, then check with Get-TransportConfig.
# Put the original values back before you finish: destroying this resource
# only removes it from the state.
resource "msexchangeserver_transport_config" "this" {
  # max_send_size_mb             = 150   # megabytes, 0 = Unlimited
  # max_receive_size_mb          = 150
  # max_recipient_envelope_limit = 1000  # 0 = Unlimited
  # safety_net_hold_time_minutes = 2880  # 2 days
  # internal_smtp_servers        = ["10.0.0.25"]
  # dsn_conversion_mode          = "UseExchangeDSNs"
}
