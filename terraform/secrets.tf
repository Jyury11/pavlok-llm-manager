# Secret Manager secrets

resource "google_secret_manager_secret" "line_channel_secret" {
  secret_id = "${var.app_name}-line-channel-secret"

  replication {
    auto {}
  }

  depends_on = [google_project_service.services]
}

resource "google_secret_manager_secret_version" "line_channel_secret" {
  secret      = google_secret_manager_secret.line_channel_secret.id
  secret_data = var.line_channel_secret
}

resource "google_secret_manager_secret" "line_channel_access_token" {
  secret_id = "${var.app_name}-line-channel-access-token"

  replication {
    auto {}
  }

  depends_on = [google_project_service.services]
}

resource "google_secret_manager_secret_version" "line_channel_access_token" {
  secret      = google_secret_manager_secret.line_channel_access_token.id
  secret_data = var.line_channel_access_token
}

resource "google_secret_manager_secret" "allowed_line_user_id" {
  secret_id = "${var.app_name}-allowed-line-user-id"

  replication {
    auto {}
  }

  depends_on = [google_project_service.services]
}

resource "google_secret_manager_secret_version" "allowed_line_user_id" {
  secret      = google_secret_manager_secret.allowed_line_user_id.id
  secret_data = var.allowed_line_user_id
}

resource "google_secret_manager_secret" "gemini_api_key" {
  secret_id = "${var.app_name}-gemini-api-key"

  replication {
    auto {}
  }

  depends_on = [google_project_service.services]
}

resource "google_secret_manager_secret_version" "gemini_api_key" {
  secret      = google_secret_manager_secret.gemini_api_key.id
  secret_data = var.gemini_api_key
}

resource "google_secret_manager_secret" "pavlok_access_token" {
  secret_id = "${var.app_name}-pavlok-access-token"

  replication {
    auto {}
  }

  depends_on = [google_project_service.services]
}

resource "google_secret_manager_secret_version" "pavlok_access_token" {
  secret      = google_secret_manager_secret.pavlok_access_token.id
  secret_data = var.pavlok_access_token
}

# IAM for Cloud Run to access secrets
resource "google_secret_manager_secret_iam_member" "cloud_run_access" {
  for_each = {
    line_channel_secret       = google_secret_manager_secret.line_channel_secret.id
    line_channel_access_token = google_secret_manager_secret.line_channel_access_token.id
    allowed_line_user_id      = google_secret_manager_secret.allowed_line_user_id.id
    gemini_api_key            = google_secret_manager_secret.gemini_api_key.id
    pavlok_access_token       = google_secret_manager_secret.pavlok_access_token.id
  }

  secret_id = each.value
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cloud_run.email}"
}
