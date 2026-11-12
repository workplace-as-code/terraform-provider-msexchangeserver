data "msexchangeserver_pop_settings" "ex01" {
  name = "EX01"
}

output "ex01_pop_login_type" {
  description = "Sign-in method of POP3 on EX01."
  value       = data.msexchangeserver_pop_settings.ex01.login_type
}
