data "msexchangeserver_organization_config" "org" {}

output "organization_name" {
  description = "Display name of the Exchange organization."
  value       = data.msexchangeserver_organization_config.org.display_name
}
