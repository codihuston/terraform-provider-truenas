# Export a dataset over NFS to a single subnet
resource "truenas_dataset" "media" {
  pool = "tank"
  path = "media"
}

resource "truenas_share_nfs" "media" {
  path    = truenas_dataset.media.full_path
  comment = "Media library"

  networks = ["10.0.0.0/24"]
  security = ["SYS"]

  maproot_user  = "root"
  maproot_group = "root"
}
