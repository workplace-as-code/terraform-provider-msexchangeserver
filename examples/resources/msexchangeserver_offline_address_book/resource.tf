# The offline address book of the Contoso GAL, distributed by every OAB virtual directory.
resource "msexchangeserver_offline_address_book" "contoso" {
  name                            = "Contoso OAB"
  address_lists                   = ["Contoso GAL"]
  global_web_distribution_enabled = true
}
