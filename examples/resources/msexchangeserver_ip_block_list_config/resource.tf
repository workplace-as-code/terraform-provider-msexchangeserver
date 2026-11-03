resource "msexchangeserver_ip_block_list_config" "this" {
  enabled                         = true
  static_entry_rejection_response = "Connection refused by Contoso."
}
