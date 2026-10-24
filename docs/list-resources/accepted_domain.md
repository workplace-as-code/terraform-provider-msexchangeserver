---
page_title: "msexchangeserver_accepted_domain (List Resource) - msexchangeserver"
subcategory: ""
description: |-
  Lists the existing msexchangeserver_accepted_domain objects, for terraform query.
---

# msexchangeserver_accepted_domain (List Resource)

Lists the existing objects of this type, built-in ones included, for `terraform query` (Terraform 1.14 and later). The list block takes no arguments.

Each result is an object that the [`msexchangeserver_accepted_domain`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/accepted_domain) resource can import. `terraform query -generate-config-out=generated.tf` writes the resource and `import` blocks of every result: see [Import an existing organization](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/import_existing).

## Example Usage

```terraform
list "msexchangeserver_accepted_domain" "all" {
  provider = msexchangeserver
}
```
