data "msexchangeserver_address_list" "rooms" {
  name = "All Rooms"
}

output "rooms_filter" {
  description = "Filter of the built-in room list."
  value       = data.msexchangeserver_address_list.rooms.recipient_filter
}
