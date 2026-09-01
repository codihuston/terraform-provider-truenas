# Issue an API key for a service account and hand it to a consumer
resource "truenas_api_key" "backup" {
  name     = "backup-agent"
  username = "backup"

  expires_at = "2035-01-02T15:04:05Z"
}

output "backup_api_key" {
  value     = truenas_api_key.backup.key
  sensitive = true
}
