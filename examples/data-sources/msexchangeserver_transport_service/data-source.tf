data "msexchangeserver_transport_service" "ex01" {
  name = "EX01"
}

output "ex01_message_tracking_days" {
  description = "Days the message tracking logs of EX01 are kept."
  value       = data.msexchangeserver_transport_service.ex01.message_tracking_log_max_age_days
}
