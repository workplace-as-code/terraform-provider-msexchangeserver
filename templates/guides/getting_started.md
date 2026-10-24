---
page_title: "Getting Started"
subcategory: "Guides"
---

# Getting Started

This guide walks you through setting up the Microsoft Exchange Server provider to manage an on-premises Exchange organization as code.

## Prerequisites

- A Windows machine joined to the Active Directory domain of the Exchange organization, with Terraform >= 1.5 (`terraform.exe` on the `PATH`). Windows PowerShell 5.1 is built into Windows; nothing else needs to be installed.
- The FQDN of an Exchange server, for example `exch01.contoso.local`.
- An account with the **Organization Management** role group, or equivalent.

The [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/authentication) explains these requirements and gives a command to check the connection.

## Step 1 — Configure the Provider

```terraform
# providers.tf
terraform {
  required_providers {
    msexchangeserver = {
      source  = "workplace-as-code/msexchangeserver"
      version = "~> 0.1"
    }
  }
}

provider "msexchangeserver" {
  server   = var.server
  username = var.username
  password = var.password
}
```

Declare the variables used by the provider block:

```terraform
# variables.tf
variable "server" {
  type = string
}

variable "username" {
  type = string
}

variable "password" {
  type      = string
  sensitive = true
}
```

Give them values in a `terraform.tfvars` file, or with `TF_VAR_<name>` environment variables. Keep `terraform.tfvars` out of version control: it holds the password.

```terraform
# terraform.tfvars
server   = "exch01.contoso.local"
username = "CONTOSO\\svc-terraform"
password = "your-password"
```

Or set everything through the `EXCHANGE_SERVER`, `EXCHANGE_USERNAME` and `EXCHANGE_PASSWORD` environment variables and leave the provider block empty: `provider "msexchangeserver" {}`. In that case, skip `variables.tf` and `terraform.tfvars`.

## Step 2 — Your First Resource

```terraform
# main.tf
resource "msexchangeserver_remote_domain" "partner" {
  name                 = "Partner"
  domain_name          = "partner.example"
  auto_forward_enabled = false
  allowed_oof_type     = "External"
}
```

```powershell
terraform init
terraform plan    # Preview — nothing is changed yet
terraform apply   # Apply to your organization
```

The first operation takes a few seconds longer: the provider opens the remote PowerShell session and imports the Exchange cmdlets. The following operations reuse the open sessions.

## Next Steps

- [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/authentication): requirements, service account, secrets
- [Import an existing organization](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/import_existing): generate the configuration of the objects already in place
- [Detect drift in CI](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/drift_detection): run `terraform plan` on a schedule to detect changes made outside Terraform
- [Resources and availability](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/availability): every resource and what has been tested
