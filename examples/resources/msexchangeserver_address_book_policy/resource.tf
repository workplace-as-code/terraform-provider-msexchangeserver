# The address book of the Contoso entity: its GAL, OAB, room list and site lists.
resource "msexchangeserver_address_book_policy" "contoso" {
  name                 = "Contoso ABP"
  global_address_list  = "Contoso GAL"
  offline_address_book = "Contoso OAB"
  room_list            = "Contoso Rooms"
  address_lists        = ["Paris"]
}
