# Always accept the mail servers of a partner.
resource "msexchangeserver_ip_allow_list_entry" "partner" {
  name    = "192.0.2.0/24"
  comment = "Fabrikam mail servers"
}
