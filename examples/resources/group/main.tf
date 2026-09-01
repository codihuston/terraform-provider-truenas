# A group to share ownership of a project directory
resource "truenas_group" "developers" {
  name = "developers"
}

# A group whose members may restart one service without a password prompt
resource "truenas_group" "operators" {
  name = "operators"

  sudo_commands_nopasswd = [
    "/usr/bin/systemctl restart myapp",
  ]
}
