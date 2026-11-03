# Block an address until the end of the year.
resource "msexchangeserver_ip_block_list_entry" "abuser" {
  name            = "198.51.100.7"
  comment         = "Spam campaign, ticket 4512"
  expiration_time = "2027-01-01T00:00:00Z"
}
