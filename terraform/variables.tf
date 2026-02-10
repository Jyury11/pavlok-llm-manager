variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "asia-northeast1"
}

variable "app_name" {
  description = "Application name"
  type        = string
  default     = "pavlok-llm-manager"
}

variable "line_channel_secret" {
  description = "LINE Channel Secret"
  type        = string
  sensitive   = true
}

variable "line_channel_access_token" {
  description = "LINE Channel Access Token"
  type        = string
  sensitive   = true
}

variable "allowed_line_user_id" {
  description = "Allowed LINE User ID"
  type        = string
  sensitive   = true
}

variable "gemini_api_key" {
  description = "Gemini API Key"
  type        = string
  sensitive   = true
}

variable "pavlok_access_token" {
  description = "Pavlok Access Token"
  type        = string
  sensitive   = true
}

variable "max_daily_shocks" {
  description = "Maximum daily shocks"
  type        = number
  default     = 10
}

variable "max_hourly_shocks" {
  description = "Maximum hourly shocks"
  type        = number
  default     = 3
}

variable "quiet_hours_start" {
  description = "Quiet hours start (hour)"
  type        = number
  default     = 23
}

variable "quiet_hours_end" {
  description = "Quiet hours end (hour)"
  type        = number
  default     = 6
}

variable "image_tag" {
  description = "Docker image tag to deploy"
  type        = string
  default     = "latest"
}

variable "review_hour" {
  description = "Daily review notification hour (24-hour format)"
  type        = number
  default     = 21
}

variable "review_minute" {
  description = "Daily review notification minute"
  type        = number
  default     = 0
}

variable "review_enable" {
  description = "Enable daily review feature"
  type        = bool
  default     = true
}

variable "morning_prompt_hour" {
  description = "Morning prompt notification hour (24-hour format)"
  type        = number
  default     = 8
}

variable "morning_prompt_minute" {
  description = "Morning prompt notification minute"
  type        = number
  default     = 15
}

variable "morning_prompt_enable" {
  description = "Enable morning prompt feature"
  type        = bool
  default     = true
}
