# Push nightly snapshots of an archive dataset to a backup host over SSH.
#
# The SSH connection is a keychain credential of type SSH_CREDENTIALS. Manage it
# with truenas_ssh_credential and pass its id, or reference the numeric ID of a
# credential created outside Terraform, as this example does.
variable "backup_ssh_credential" {
  description = "ID of the SSH_CREDENTIALS keychain credential for the backup host."
  type        = number
}

resource "truenas_dataset" "archive" {
  pool = "tank"
  path = "archive"
}

resource "truenas_replication_task" "archive_to_backup" {
  name            = "archive to backup"
  ssh_credentials = var.backup_ssh_credential

  source_datasets = ["${truenas_dataset.archive.pool}/${truenas_dataset.archive.path}"]
  target_dataset  = "backup/archive"
  recursive       = true

  # Replicate the snapshots a periodic snapshot task produces.
  also_include_naming_schema = ["auto-%Y-%m-%d_%H-%M"]

  retention_policy = "CUSTOM"
  lifetime_value   = 2
  lifetime_unit    = "WEEK"

  compression = "LZ4"

  schedule {
    minute = "0"
    hour   = "3"
  }
}
