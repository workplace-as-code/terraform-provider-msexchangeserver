# IMAP4 on EX01: encrypted connections only, with the certificate of mail.contoso.com.
resource "msexchangeserver_imap_settings" "ex01" {
  name                         = "EX01"
  login_type                   = "SecureLogin"
  x509_certificate_name        = "mail.contoso.com"
  internal_connection_settings = ["mail.contoso.com:993:SSL"]
  ssl_bindings                 = ["0.0.0.0:993", "[::]:993"]
  max_connections_per_user     = 4
}
