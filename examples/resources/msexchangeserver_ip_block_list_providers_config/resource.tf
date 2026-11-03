# Query the DNS block lists, but never for the postmaster.
resource "msexchangeserver_ip_block_list_providers_config" "this" {
  enabled             = true
  bypassed_recipients = ["postmaster@contoso.com"]
}
