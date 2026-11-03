# Outgoing mail to the internet through the mail gateways, over TLS.
resource "msexchangeserver_send_connector" "internet" {
  name                     = "To internet via gateways"
  address_spaces           = ["*"]
  smart_hosts              = ["gw1.contoso.com", "gw2.contoso.com"]
  dns_routing_enabled      = false
  source_transport_servers = ["EX01", "EX02"]
  require_tls              = true
  tls_auth_level           = "CertificateValidation"
}
