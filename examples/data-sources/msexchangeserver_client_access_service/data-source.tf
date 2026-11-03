data "msexchangeserver_client_access_service" "ex01" {
  name = "EX01"
}

output "autodiscover_service_internal_uri" {
  description = "Autodiscover URL that EX01 publishes in Active Directory."
  value       = data.msexchangeserver_client_access_service.ex01.autodiscover_service_internal_uri
}
