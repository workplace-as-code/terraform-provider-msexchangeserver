# Domain-joined Outlook clients find Autodiscover through Active Directory.
locals {
  autodiscover_url = "https://autodiscover.contoso.com/Autodiscover/Autodiscover.xml"
}

resource "msexchangeserver_client_access_service" "ex01" {
  name                              = "EX01"
  autodiscover_service_internal_uri = local.autodiscover_url
}
