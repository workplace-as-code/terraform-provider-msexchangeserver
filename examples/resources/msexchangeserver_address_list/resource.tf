# The users of the Paris office, as a list nested in a "Sites" list.
resource "msexchangeserver_address_list" "paris" {
  name             = "Paris"
  container        = "Sites"
  recipient_filter = "Office -eq 'Paris' -and RecipientType -eq 'UserMailbox'"
}

# The room mailboxes, for the room list of an address book policy.
resource "msexchangeserver_address_list" "contoso_rooms" {
  name                = "Contoso Rooms"
  recipient_filter    = "RecipientDisplayType -eq 'ConferenceRoomMailbox'"
  recipient_container = "contoso.com/Contoso"
}
