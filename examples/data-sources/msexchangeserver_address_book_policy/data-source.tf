data "msexchangeserver_address_book_policy" "contoso" {
  name = "Contoso ABP"
}

output "contoso_gal" {
  description = "GAL of the Contoso address book policy."
  value       = data.msexchangeserver_address_book_policy.contoso.global_address_list
}
