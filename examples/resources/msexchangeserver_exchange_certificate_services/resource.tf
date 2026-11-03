# A certificate already on the server (requested and completed in the
# Exchange admin center), enabled for HTTPS, SMTP, IMAP and POP.
resource "msexchangeserver_exchange_certificate_services" "mail" {
  server     = "EX01.contoso.com"
  thumbprint = "0123456789ABCDEF0123456789ABCDEF01234567"
  services   = ["IIS", "SMTP", "IMAP", "POP"]
}
