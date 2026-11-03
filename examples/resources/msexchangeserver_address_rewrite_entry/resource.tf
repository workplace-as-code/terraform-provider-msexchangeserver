# Mail of the subsidiary leaves with the addresses of the group.
resource "msexchangeserver_address_rewrite_entry" "subsidiary" {
  name             = "Fabrikam to Contoso"
  internal_address = "*.fabrikam.com"
  external_address = "contoso.com"
  outbound_only    = true
}
