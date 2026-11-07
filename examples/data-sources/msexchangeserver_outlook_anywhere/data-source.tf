data "msexchangeserver_outlook_anywhere" "ex01" {
  name = "EX01"
}

output "external_hostname" {
  description = "External host name of Outlook Anywhere on EX01."
  value       = data.msexchangeserver_outlook_anywhere.ex01.external_hostname
}
