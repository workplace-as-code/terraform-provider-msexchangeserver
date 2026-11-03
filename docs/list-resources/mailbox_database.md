---
page_title: "msexchangeserver_mailbox_database (List Resource) - msexchangeserver"
subcategory: ""
description: |-
  Lists the existing msexchangeserver_mailbox_database objects, for terraform query.
---

# msexchangeserver_mailbox_database (List Resource)

Lists the existing objects of this type, built-in ones included, for `terraform query` (Terraform 1.14 and later). The list block takes no arguments.

Each result is an object that the [`msexchangeserver_mailbox_database`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/mailbox_database) resource can import. `terraform query -generate-config-out=generated.tf` writes the resource and `import` blocks of every result: see [Import an existing organization](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/import_existing).

## Example Usage

```terraform
list "msexchangeserver_mailbox_database" "all" {
  provider = msexchangeserver
}
```
