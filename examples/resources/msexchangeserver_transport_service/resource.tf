# Message tracking logs of EX01 kept 90 days on a dedicated disk.
resource "msexchangeserver_transport_service" "ex01" {
  name                                       = "EX01"
  message_tracking_log_path                  = "D:\\Logs\\MessageTracking"
  message_tracking_log_max_age_days          = 90
  message_tracking_log_max_directory_size_mb = 10240
  connectivity_log_max_age_days              = 90
  send_protocol_log_max_age_days             = 90
  receive_protocol_log_max_age_days          = 90
}
