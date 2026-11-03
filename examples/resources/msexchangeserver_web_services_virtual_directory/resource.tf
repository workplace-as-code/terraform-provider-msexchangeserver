# The same namespace inside and outside the network.
resource "msexchangeserver_web_services_virtual_directory" "ex01" {
  name         = "EX01"
  internal_url = "https://mail.contoso.com/EWS/Exchange.asmx"
  external_url = "https://mail.contoso.com/EWS/Exchange.asmx"
}
