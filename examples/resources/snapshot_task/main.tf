# Snapshot a dataset and its children every night at 02:00, keeping two weeks
resource "truenas_dataset" "data" {
  pool = "tank"
  path = "data"
}

resource "truenas_snapshot_task" "nightly" {
  dataset   = truenas_dataset.data.id
  recursive = true

  naming_schema  = "nightly-%Y-%m-%d_%H-%M"
  lifetime_value = 2
  lifetime_unit  = "WEEK"

  schedule = {
    minute = "0"
    hour   = "2"
  }
}
