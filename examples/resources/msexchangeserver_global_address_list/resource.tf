# The GAL of the Contoso entity: its recipients carry Contoso in CustomAttribute1.
resource "msexchangeserver_global_address_list" "contoso" {
  name             = "Contoso GAL"
  recipient_filter = "CustomAttribute1 -eq 'Contoso'"
}
