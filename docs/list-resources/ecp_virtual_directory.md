---
page_title: "msexchangeserver_ecp_virtual_directory (List Resource) - msexchangeserver"
subcategory: ""
description: |-
  Lists the existing msexchangeserver_ecp_virtual_directory objects, for terraform query.
---

# msexchangeserver_ecp_virtual_directory (List Resource)

Lists the existing objects of this type, built-in ones included, for `terraform query` (Terraform 1.14 and later). The list block takes no arguments.

Each result is an object that the [`msexchangeserver_ecp_virtual_directory`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/ecp_virtual_directory) resource can import. `terraform query -generate-config-out=generated.tf` writes the resource and `import` blocks of every result: see [Import an existing organization](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/import_existing).

## Example Usage

```terraform
list "msexchangeserver_ecp_virtual_directory" "all" {
  provider = msexchangeserver
}
```
