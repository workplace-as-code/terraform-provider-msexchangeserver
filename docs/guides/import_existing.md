---
page_title: "Import an existing organization"
subcategory: "Guides"
---

# Import an existing organization

An organization in production already has policies, rules and domains. Terraform 1.14 and later can find them through the provider and write their configuration for you, so you start from what is configured today instead of rewriting it by hand.

Every resource has a **list resource** of the same name that returns the existing objects of that type. `terraform query` runs the list resources, and `-generate-config-out` turns the results into resource and `import` blocks.

## 1. List the objects

Next to your provider configuration, create a file whose name ends in `.tfquery.hcl`, with one `list` block per resource type to import:

```terraform
# export.tfquery.hcl
list "msexchangeserver_transport_rule" "all" {
  provider = msexchangeserver
}

list "msexchangeserver_owa_mailbox_policy" "all" {
  provider = msexchangeserver
}

list "msexchangeserver_remote_domain" "all" {
  provider = msexchangeserver
}
```

A `list` block takes no arguments: it returns every object of that type, built-in ones included (default policies, built-in retention tags, ...). Organization-wide settings such as `msexchangeserver_organization_config` return their single object.

```shell
terraform init
terraform query
```

`terraform query` prints the objects found. It reads the organization and changes nothing.

## 2. Generate the configuration

```shell
terraform query -generate-config-out=generated.tf
```

`generated.tf` holds a `resource` block with the current values and an `import` block for each object.

## 3. Review, then import

```shell
terraform plan
```

The plan must only show imports: `Plan: N to import, 0 to add, 0 to change, 0 to destroy.` `terraform apply` then records the objects in the state, without changing Exchange.

Review `generated.tf` before applying:

- **Remove what Terraform should not manage.** Built-in objects (the `Default` policies, the default email address policy, the built-in retention policy and retention tags) cannot be deleted: `terraform destroy` fails on them. Keep them only if you want to manage their settings, ideally with `lifecycle { prevent_destroy = true }`. Organization-wide settings are never deleted: destroying them only removes them from the state.
- **Remove the attributes you do not want to manage.** The generated blocks set every attribute. Resources only manage the attributes present in the configuration, so deleting a line leaves that setting to the admin center.
- **List one resource per object.** A policy and its rule can be managed together by the policy resource (through its recipient conditions) or apart with the rule resource: list one or the other, not both.
- **Rename the resources.** Terraform names them `<list name>_<n>`. Rename them in the `resource` block and in the `to` of the `import` block before applying, or with `moved` blocks afterwards.


## Import a single object

An `import` block takes the object's ID or its identity (Terraform 1.12 and later):

```terraform
import {
  to       = msexchangeserver_transport_rule.block_executables
  identity = { id = "Block executable attachments" }
}
```

`terraform plan -generate-config-out=generated.tf` writes the configuration of the objects in `import` blocks that have no `resource` block yet. Each resource page lists its import ID.
