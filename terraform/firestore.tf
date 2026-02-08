# Firestore database

resource "google_firestore_database" "main" {
  provider    = google-beta
  project     = var.project_id
  name        = "(default)"
  location_id = var.region
  type        = "FIRESTORE_NATIVE"

  depends_on = [google_project_service.services]
}

# Firestore indexes for efficient queries

resource "google_firestore_index" "schedules_status_created" {
  provider   = google-beta
  project    = var.project_id
  database   = google_firestore_database.main.name
  collection = "schedules"

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }

  fields {
    field_path = "created_at"
    order      = "ASCENDING"
  }

  fields {
    field_path = "deadline"
    order      = "ASCENDING"
  }
}

resource "google_firestore_index" "schedules_created_deadline" {
  provider   = google-beta
  project    = var.project_id
  database   = google_firestore_database.main.name
  collection = "schedules"

  fields {
    field_path = "created_at"
    order      = "ASCENDING"
  }

  fields {
    field_path = "deadline"
    order      = "ASCENDING"
  }
}

resource "google_firestore_index" "punishment_logs_executed" {
  provider   = google-beta
  project    = var.project_id
  database   = google_firestore_database.main.name
  collection = "punishment_logs"

  fields {
    field_path = "executed_at"
    order      = "ASCENDING"
  }
}

resource "google_firestore_index" "daily_stats_date" {
  provider   = google-beta
  project    = var.project_id
  database   = google_firestore_database.main.name
  collection = "daily_stats"

  fields {
    field_path = "date"
    order      = "DESCENDING"
  }
}
