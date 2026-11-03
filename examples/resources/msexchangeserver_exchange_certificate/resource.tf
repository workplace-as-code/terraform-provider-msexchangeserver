variable "mail_pfx_password" {
  description = "Password of the PFX file. Ephemeral: never stored in a plan or the state."
  type        = string
  sensitive   = true
  ephemeral   = true
}

# The certificate of mail.contoso.com, imported from a PFX file on the
# machine that runs Terraform, then enabled for IIS and SMTP.
resource "msexchangeserver_exchange_certificate" "mail_2027" {
  server          = "EX01.contoso.com"
  thumbprint      = "0123456789ABCDEF0123456789ABCDEF01234567"
  pfx_base64_wo   = filebase64("${path.module}/mail-2027.pfx")
  pfx_password_wo = var.mail_pfx_password
  friendly_name   = "mail.contoso.com 2027"
}

resource "msexchangeserver_exchange_certificate_services" "mail_2027" {
  server     = msexchangeserver_exchange_certificate.mail_2027.server
  thumbprint = msexchangeserver_exchange_certificate.mail_2027.thumbprint
  services   = ["IIS", "SMTP"]
}
