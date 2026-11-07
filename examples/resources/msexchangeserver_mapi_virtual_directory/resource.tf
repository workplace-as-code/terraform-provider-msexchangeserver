# The same namespace inside and outside the network, with OAuth for hybrid
# modern authentication.
resource "msexchangeserver_mapi_virtual_directory" "ex01" {
  name         = "EX01"
  internal_url = "https://mail.contoso.com/mapi"
  external_url = "https://mail.contoso.com/mapi"

  iis_authentication_methods = ["Ntlm", "OAuth", "Negotiate"]
}
