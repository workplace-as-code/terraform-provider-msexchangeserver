data "msexchangeserver_accepted_domains" "all" {}

output "authoritative_domains" {
  description = "Names of the authoritative accepted domains of the organization."
  value       = [for d in data.msexchangeserver_accepted_domains.all.domains : d.domain_name if d.domain_type == "Authoritative"]
}
