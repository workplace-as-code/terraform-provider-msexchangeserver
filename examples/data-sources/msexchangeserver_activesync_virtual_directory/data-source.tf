data "msexchangeserver_activesync_virtual_directory" "ex01" {
  name = "EX01"
}

output "external_url" {
  description = "External URL of the virtual directory of EX01."
  value       = data.msexchangeserver_activesync_virtual_directory.ex01.external_url
}
