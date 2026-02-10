# Service Account for Cloud Run
resource "google_service_account" "cloud_run" {
  account_id   = "${var.app_name}-run-sa"
  display_name = "Service Account for ${var.app_name} Cloud Run"
}

# IAM roles for Service Account
resource "google_project_iam_member" "cloud_run_firestore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.cloud_run.email}"
}

# Cloud Run service
resource "google_cloud_run_v2_service" "app" {
  name     = var.app_name
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = google_service_account.cloud_run.email

    scaling {
      min_instance_count = 0
      max_instance_count = 2
    }

    containers {
      image = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.app.repository_id}/${var.app_name}:${var.image_tag}"

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
      }

      # Environment variables
      env {
        name  = "DB_TYPE"
        value = "firestore"
      }

      env {
        name  = "GCP_PROJECT_ID"
        value = var.project_id
      }

      env {
        name  = "PORT"
        value = "8080"
      }

      env {
        name  = "MAX_DAILY_SHOCKS"
        value = tostring(var.max_daily_shocks)
      }

      env {
        name  = "MAX_HOURLY_SHOCKS"
        value = tostring(var.max_hourly_shocks)
      }

      env {
        name  = "SHOCK_LEVEL"
        value = "50"
      }

      env {
        name  = "QUIET_HOURS_START"
        value = tostring(var.quiet_hours_start)
      }

      env {
        name  = "QUIET_HOURS_END"
        value = tostring(var.quiet_hours_end)
      }

      env {
        name  = "REVIEW_HOUR"
        value = tostring(var.review_hour)
      }

      env {
        name  = "REVIEW_MINUTE"
        value = tostring(var.review_minute)
      }

      env {
        name  = "REVIEW_ENABLE"
        value = var.review_enable ? "true" : "false"
      }

      env {
        name  = "MORNING_PROMPT_HOUR"
        value = tostring(var.morning_prompt_hour)
      }

      env {
        name  = "MORNING_PROMPT_MINUTE"
        value = tostring(var.morning_prompt_minute)
      }

      env {
        name  = "MORNING_PROMPT_ENABLE"
        value = var.morning_prompt_enable ? "true" : "false"
      }

      # Secrets
      env {
        name = "LINE_CHANNEL_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.line_channel_secret.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "LINE_CHANNEL_ACCESS_TOKEN"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.line_channel_access_token.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "ALLOWED_LINE_USER_ID"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.allowed_line_user_id.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "GEMINI_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.gemini_api_key.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "PAVLOK_ACCESS_TOKEN"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.pavlok_access_token.secret_id
            version = "latest"
          }
        }
      }

      startup_probe {
        http_get {
          path = "/health"
          port = 8080
        }
        initial_delay_seconds = 5
        timeout_seconds       = 3
        period_seconds        = 10
        failure_threshold     = 3
      }

      liveness_probe {
        http_get {
          path = "/health"
          port = 8080
        }
        timeout_seconds   = 3
        period_seconds    = 30
        failure_threshold = 3
      }
    }
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  depends_on = [
    google_project_service.services,
    google_secret_manager_secret_iam_member.cloud_run_access,
    google_firestore_database.main,
  ]
}

# Allow unauthenticated access (for LINE webhook)
resource "google_cloud_run_v2_service_iam_member" "allow_unauthenticated" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.app.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
