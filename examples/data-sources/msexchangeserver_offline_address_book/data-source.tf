data "msexchangeserver_offline_address_book" "default" {
  name = "Default Offline Address Book"
}

output "default_oab_lists" {
  description = "Address lists of the default offline address book."
  value       = data.msexchangeserver_offline_address_book.default.address_lists
}
