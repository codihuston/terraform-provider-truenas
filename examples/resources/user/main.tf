resource "truenas_group" "developers" {
  name = "developers"
}

# A service account that authenticates with an SSH key and has no password
resource "truenas_user" "deploy" {
  username  = "deploy"
  full_name = "Deployment Account"

  group  = truenas_group.developers.id
  groups = [truenas_group.developers.id]

  home        = "/mnt/tank/home"
  home_create = true
  shell       = "/usr/bin/bash"

  password_disabled = true
  ssh_public_key    = file("${path.module}/deploy.pub")

  sudo_commands_nopasswd = [
    "/usr/bin/systemctl restart myapp",
  ]
}

# An account with its own primary group, created and deleted alongside the user
resource "truenas_user" "alice" {
  username  = "alice"
  full_name = "Alice Example"

  group_create      = true
  shell             = "/usr/bin/zsh"
  password_disabled = true
}
