resource "msexchangeserver_accepted_domain" "contoso_com" {
  name        = "Contoso"
  domain_name = "contoso.com"
  domain_type = "Authoritative"
}

resource "msexchangeserver_accepted_domain" "subsidiary_relay" {
  name        = "Subsidiary Internal Relay"
  domain_name = "subsidiary.contoso.com"
  domain_type = "InternalRelay"
}
