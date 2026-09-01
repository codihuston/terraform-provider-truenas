resource "tls_private_key" "backup" {
  algorithm = "ED25519"
}

resource "truenas_ssh_keypair" "backup" {
  name = "backup-host"

  private_key            = tls_private_key.backup.private_key_openssh
  private_key_wo_version = 1
}

# The host key is discovered when the connection is created, so the remote host
# must be reachable at apply time.
resource "truenas_ssh_credential" "backup" {
  name = "backup-host"

  host           = "backup.example.com"
  username       = "truenas_replication"
  private_key_id = truenas_ssh_keypair.backup.id
}
