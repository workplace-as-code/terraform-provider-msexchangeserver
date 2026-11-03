data "msexchangeserver_retention_policy" "this" {
  name = "Default MRM Policy"
}

output "retention_policy_tag_links" {
  description = "Tags of the default retention policy."
  value       = data.msexchangeserver_retention_policy.this.retention_policy_tag_links
}
