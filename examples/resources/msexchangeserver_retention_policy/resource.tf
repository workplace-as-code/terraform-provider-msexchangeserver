resource "msexchangeserver_retention_policy" "standard" {
  name = "Standard Retention Policy"
  retention_policy_tag_links = [
    "Default 2 year move to archive",
    "Recoverable Items 14 days move to archive",
    "1 Month Delete",
    "1 Week Delete",
  ]
}

resource "msexchangeserver_retention_policy" "compliance_7yr" {
  name = "7-Year Compliance Policy"
  retention_policy_tag_links = [
    "7 Year Delete",
    "Recoverable Items 14 days move to archive",
  ]
}
