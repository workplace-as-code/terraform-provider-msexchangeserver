# One namespace for every client, Negotiate (Kerberos, then NTLM) everywhere.
resource "msexchangeserver_outlook_anywhere" "ex01" {
  name              = "EX01"
  internal_hostname = "mail.contoso.com"
  external_hostname = "mail.contoso.com"

  internal_client_authentication_method = "Negotiate"
  external_client_authentication_method = "Negotiate"
  iis_authentication_methods            = ["Basic", "Ntlm", "Negotiate"]
  internal_clients_require_ssl          = true
  external_clients_require_ssl          = true
}
