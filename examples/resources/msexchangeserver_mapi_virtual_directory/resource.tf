# The same namespace inside and outside the network.
resource "msexchangeserver_mapi_virtual_directory" "ex01" {
  name         = "EX01"
  internal_url = "https://mail.contoso.com/mapi"
  external_url = "https://mail.contoso.com/mapi"
}
