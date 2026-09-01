# Generate the key pair with the tls provider so Terraform owns it, and hand the
# private half to TrueNAS. The public half goes in the remote account's
# authorized_keys.
resource "tls_private_key" "backup" {
  algorithm = "ED25519"
}

resource "truenas_ssh_keypair" "backup" {
  name = "backup-host"

  private_key            = tls_private_key.backup.private_key_openssh
  private_key_wo_version = 1
}
