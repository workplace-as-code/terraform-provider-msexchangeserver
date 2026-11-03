# Relay for two application servers, without Active Directory permissions:
# the connector treats them as Exchange servers (externally secured).
resource "msexchangeserver_receive_connector" "app_relay" {
  name              = "EX01\\Application relay"
  usage             = "Custom"
  transport_role    = "FrontendTransport"
  bindings          = ["0.0.0.0:25"]
  remote_ip_ranges  = ["10.10.5.21", "10.10.5.22"]
  permission_groups = ["ExchangeServers"]
  auth_mechanism    = ["Tls", "ExternalAuthoritative"]
}
