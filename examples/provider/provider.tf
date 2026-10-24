terraform {
  required_providers {
    msexchangeserver = {
      source  = "workplace-as-code/msexchangeserver"
      version = "~> 0.1"
    }
  }
}

# Terraform runs on a Windows machine joined to the domain: the provider opens
# an Exchange remote PowerShell session with Kerberos.
provider "msexchangeserver" {
  server   = "exch01.contoso.local"
  username = var.username
  password = var.password
}

resource "msexchangeserver_remote_domain" "partner" {
  name                 = "Partner"
  domain_name          = "partner.example"
  auto_forward_enabled = false
}
