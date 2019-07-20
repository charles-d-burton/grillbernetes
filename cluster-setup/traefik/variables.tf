#Boiler Plate Variables
variable "domain" {
  description = "Primary ingress domain for this environment"
  type        = string
  default     = "home.rsmachiner.com"
}

variable "namespace" {
  description = "The namespace to deploy traefik into"
  default     = "kube-system"
}

variable "service_name" {
  description = "Name of the service"
  default     = "traefik-lb"
}

variable "ingress_label" {
  description = "Labels for deployment/daemonset"
  default     = "traefik-lb-ingress"
}

variable "ingress_class" {
  description = "Class annotation for ingress"
  default     = "traefik"
}

variable "env" {
  description = "The deployment environment"
  default     = "home"
}

variable "internal" {
  description = "Whether or not it should be exposed to the internet"
  default     = true
}

variable "admin_port" {
  description = "Port for the admin ui"
  default     = 8080
}

