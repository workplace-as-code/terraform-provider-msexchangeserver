data "msexchangeserver_exchange_certificate" "mail" {
  server     = "EX01.contoso.com"
  thumbprint = "0123456789ABCDEF0123456789ABCDEF01234567"
}

output "mail_certificate_expires" {
  description = "End of validity of the certificate, in UTC."
  value       = data.msexchangeserver_exchange_certificate.mail.not_after
}
