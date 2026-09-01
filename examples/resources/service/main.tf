# Turn on NFS so the shares below can be mounted
resource "truenas_service" "nfs" {
  name = "nfs"
}

resource "truenas_dataset" "media" {
  pool = "tank"
  path = "media"
}

resource "truenas_share_nfs" "media" {
  path    = truenas_dataset.media.full_path
  comment = "Media library"

  depends_on = [truenas_service.nfs]
}
