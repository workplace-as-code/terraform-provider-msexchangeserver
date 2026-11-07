# Every server gets the same namespace and the same authentication settings:
# a new server is aligned on apply, and a server that differs shows in the plan.
resource "msexchangeserver_web_services_virtual_directory" "this" {
  for_each = toset(["EX01", "EX02"])

  name         = each.key
  internal_url = "https://mail.contoso.com/EWS/Exchange.asmx"
  external_url = "https://mail.contoso.com/EWS/Exchange.asmx"

  basic_authentication = false
  oauth_authentication = true
}
