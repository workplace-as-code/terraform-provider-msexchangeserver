data "msexchangeserver_accepted_domain" "primary" {
  name = "contoso.com"
}

output "primary_domain_type" {
  description = "Type of contoso.com: Authoritative, InternalRelay or ExternalRelay."
  value       = data.msexchangeserver_accepted_domain.primary.domain_type
}
