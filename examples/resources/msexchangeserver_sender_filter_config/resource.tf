# Block mail without a sender and from two domains.
resource "msexchangeserver_sender_filter_config" "this" {
  blank_sender_blocking_enabled  = true
  blocked_domains_and_subdomains = ["spam.example", "phish.example"]
}
