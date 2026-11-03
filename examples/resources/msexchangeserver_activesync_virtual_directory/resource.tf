# The same namespace inside and outside the network.
resource "msexchangeserver_activesync_virtual_directory" "ex01" {
  name         = "EX01"
  internal_url = "https://mail.contoso.com/Microsoft-Server-ActiveSync"
  external_url = "https://mail.contoso.com/Microsoft-Server-ActiveSync"
}
