data "msexchangeserver_frontend_transport_service" "ex01" {
  name = "EX01"
}

output "ex01_frontend_receive_log_path" {
  description = "Folder of the Front End Transport receive protocol logs of EX01."
  value       = data.msexchangeserver_frontend_transport_service.ex01.receive_protocol_log_path
}
