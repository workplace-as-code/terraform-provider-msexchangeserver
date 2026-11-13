# Basic authentication blocked for every protocol, set as the organization's default policy.
resource "msexchangeserver_authentication_policy" "block_basic" {
  name                                   = "Block Basic"
  block_legacy_auth_activesync           = true
  block_legacy_auth_autodiscover         = true
  block_legacy_auth_imap                 = true
  block_legacy_auth_mapi                 = true
  block_legacy_auth_offline_address_book = true
  block_legacy_auth_pop                  = true
  block_legacy_auth_rpc                  = true
  block_legacy_auth_web_services         = true
}

resource "msexchangeserver_organization_config" "this" {
  default_authentication_policy = msexchangeserver_authentication_policy.block_basic.name
}
