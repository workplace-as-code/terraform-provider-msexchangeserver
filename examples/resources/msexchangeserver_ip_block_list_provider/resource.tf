resource "msexchangeserver_ip_block_list_provider" "spamhaus" {
  name               = "Spamhaus"
  lookup_domain      = "zen.spamhaus.org"
  any_match          = true
  rejection_response = "Rejected: your address is listed by Spamhaus."
}
