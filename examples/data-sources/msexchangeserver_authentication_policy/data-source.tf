data "msexchangeserver_authentication_policy" "block_basic" {
  name = "Block Basic"
}

output "basic_pop_blocked" {
  description = "Whether the policy blocks Basic authentication for POP3."
  value       = data.msexchangeserver_authentication_policy.block_basic.block_legacy_auth_pop
}
