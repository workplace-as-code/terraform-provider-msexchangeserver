data "msexchangeserver_retention_policy_tag" "this" {
  name = "Deleted Items"
}

output "age_limit_for_retention" {
  description = "Days before deleted items are removed."
  value       = data.msexchangeserver_retention_policy_tag.this.age_limit_for_retention
}
