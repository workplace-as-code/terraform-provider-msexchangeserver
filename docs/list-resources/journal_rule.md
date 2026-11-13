---
page_title: "msexchangeserver_journal_rule (List Resource) - msexchangeserver"
subcategory: ""
description: |-
  Lists the existing msexchangeserver_journal_rule objects, for terraform query.
---

# msexchangeserver_journal_rule (List Resource)

Lists the existing objects of this type, built-in ones included, for `terraform query` (Terraform 1.14 and later). The list block takes no arguments.

Each result is an object that the [`msexchangeserver_journal_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/journal_rule) resource can import. `terraform query -generate-config-out=generated.tf` writes the resource and `import` blocks of every result: see [Import an existing organization](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/import_existing).

## Example Usage

```terraform
list "msexchangeserver_journal_rule" "all" {
  provider = msexchangeserver
}
```
