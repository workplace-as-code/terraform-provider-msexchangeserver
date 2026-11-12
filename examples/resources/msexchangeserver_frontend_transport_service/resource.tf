# Front End Transport logs of EX01 kept 90 days on a dedicated disk.
resource "msexchangeserver_frontend_transport_service" "ex01" {
  name                              = "EX01"
  connectivity_log_path             = "D:\\Logs\\FrontEnd\\Connectivity"
  connectivity_log_max_age_days     = 90
  receive_protocol_log_path         = "D:\\Logs\\FrontEnd\\SmtpReceive"
  receive_protocol_log_max_age_days = 90
}
