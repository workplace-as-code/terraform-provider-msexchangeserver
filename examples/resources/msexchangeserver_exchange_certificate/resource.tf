# The PFX and its password come from a secret store, for example through
# TF_VAR_mail_pfx_base64 and TF_VAR_mail_pfx_password in the pipeline.
# Ephemeral: they are never stored in a plan or the state.
variable "mail_pfx_base64" {
  description = "The PFX file of mail.contoso.com, encoded in base64."
  type        = string
  sensitive   = true
  ephemeral   = true
}

variable "mail_pfx_password" {
  description = "Password of the PFX file."
  type        = string
  sensitive   = true
  ephemeral   = true
}

# The certificate of mail.contoso.com, enabled for IIS and SMTP. It does not
# replace the default SMTP certificate of the server.
resource "msexchangeserver_exchange_certificate" "mail_2027" {
  server          = "EX01.contoso.com"
  thumbprint      = "0123456789ABCDEF0123456789ABCDEF01234567"
  pfx_base64_wo   = var.mail_pfx_base64
  pfx_password_wo = var.mail_pfx_password
  friendly_name   = "mail.contoso.com 2027"
}

resource "msexchangeserver_exchange_certificate_services" "mail_2027" {
  server     = msexchangeserver_exchange_certificate.mail_2027.server
  thumbprint = msexchangeserver_exchange_certificate.mail_2027.thumbprint
  services   = ["IIS", "SMTP"]
}
