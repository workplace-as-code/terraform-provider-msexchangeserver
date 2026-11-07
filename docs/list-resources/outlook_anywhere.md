---
page_title: "msexchangeserver_outlook_anywhere (List Resource) - msexchangeserver"
subcategory: ""
description: |-
  Lists the existing msexchangeserver_outlook_anywhere objects, for terraform query.
---

# msexchangeserver_outlook_anywhere (List Resource)

Lists the existing objects of this type, built-in ones included, for `terraform query` (Terraform 1.14 and later). The list block takes no arguments.

Each result is an object that the [`msexchangeserver_outlook_anywhere`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/outlook_anywhere) resource can import. `terraform query -generate-config-out=generated.tf` writes the resource and `import` blocks of every result: see [Import an existing organization](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/import_existing).

## Example Usage

```terraform
list "msexchangeserver_outlook_anywhere" "all" {
  provider = msexchangeserver
}
```
