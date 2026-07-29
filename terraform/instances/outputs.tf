output "public_ip" {
  description = "Elastic IP of the ForecastIQ origin — feeds the DNS module var.vps_ip"
  value       = aws_eip.forecastiq.public_ip
}

output "instance_id" {
  description = "EC2 instance id"
  value       = aws_instance.forecastiq.id
}

output "ssh_target" {
  description = "SSH connection string for the deploy user"
  value       = "deploy@${aws_eip.forecastiq.public_ip}"
}
