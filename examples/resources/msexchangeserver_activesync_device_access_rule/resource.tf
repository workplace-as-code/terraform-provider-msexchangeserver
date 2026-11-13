# iPhones are allowed, whatever the default access level.
resource "msexchangeserver_activesync_device_access_rule" "iphone" {
  query_string   = "iPhone"
  characteristic = "DeviceModel"
  access_level   = "Allow"
}
