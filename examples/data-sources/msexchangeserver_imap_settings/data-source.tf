data "msexchangeserver_imap_settings" "ex01" {
  name = "EX01"
}

output "ex01_imap_login_type" {
  description = "Sign-in method of IMAP4 on EX01."
  value       = data.msexchangeserver_imap_settings.ex01.login_type
}
