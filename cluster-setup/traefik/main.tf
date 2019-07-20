resource "kubernetes_service_account" "traefik_account" {
  metadata {
    name      = var.service_name
    namespace = var.namespace
  }
}

resource "kubernetes_cluster_role" "traefik_cluster_role" {
  metadata {
    name = var.service_name
  }

  rule {
    api_groups = [""]

    resources = [
      "services",
      "endpoints",
      "secrets",
    ]

    verbs = [
      "get",
      "list",
      "watch",
    ]
  }

  rule {
    api_groups = ["extensions"]
    resources  = ["ingresses"]

    verbs = [
      "get",
      "list",
      "watch",
    ]
  }

  rule {
    api_groups = ["extensions"]
    resources  = ["ingresses/status"]
    verbs      = ["update"]
  }
}

resource "kubernetes_cluster_role_binding" "traefik_cluster_role_binding" {
  metadata {
    name = var.service_name
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = var.service_name
  }

  subject {
    kind      = "ServiceAccount"
    name      = var.service_name
    namespace = var.namespace
  }

  depends_on = [kubernetes_cluster_role.traefik_cluster_role]
}

resource "kubernetes_config_map" "traefik_config_map" {
  metadata {
    name      = var.service_name
    namespace = var.namespace

    labels = {
      k8s-app = var.ingress_label
    }
  }

  data = {
    "traefik.toml" = data.template_file.traefik_config.rendered
  }
}

resource "kubernetes_deployment" "traefik_deployment" {
  metadata {
    name      = var.service_name
    namespace = var.namespace

    labels = {
      k8s-app = var.ingress_label
    }
  }

  spec {
    replicas = 3
    selector {
      match_labels = {
        k8s-app = var.ingress_label
      }
    }

    template {
      metadata {
        labels = {
          k8s-app = var.ingress_label
          name    = var.ingress_label
        }
      }

      spec {
        automount_service_account_token  = "true"
        service_account_name             = var.service_name
        termination_grace_period_seconds = 60
        container {
          image = "traefik"
          name  = var.ingress_label

          port {
            name           = "http"
            container_port = 80
          }

          port {
            name           = "https"
            container_port = 443
          }

          port {
            name           = "admin"
            container_port = 8080
          }

          args = [
            "--configfile=/config/traefik.toml",
          ]

          volume_mount {
            name       = "config"
            mount_path = "/config"
          }
        }

        volume {
          name = "config"

          config_map {
            name = var.service_name
          }
        }
      }
    }
  }

  depends_on = [kubernetes_config_map.traefik_config_map]
}

resource "kubernetes_service" "traefik_service" {
  metadata {
    name      = var.service_name
    namespace = var.namespace
  }

  spec {
    selector = {
      k8s-app = var.ingress_label
    }

    port {
      protocol    = "TCP"
      port        = "80"
      name        = "http"
      target_port = "80"
    }

    port {
      protocol    = "TCP"
      port        = "443"
      name        = "https"
      target_port = "443"
    }

    type = "LoadBalancer"
  }
}

resource "kubernetes_service" "traefik_ingress_service" {
  metadata {
    name      = "traefik-${var.ingress_class}-web-ui"
    namespace = var.namespace
  }

  spec {
    selector = {
      k8s-app = var.ingress_label
    }

    port {
      port        = "80"
      name        = "http"
      target_port = "8080"
    }

    type = "LoadBalancer"
  }
}

resource "kubernetes_secret" "admin_secret" {
  metadata {
    name      = "traefik-${var.ingress_class}-cert"
    namespace = var.namespace
  }

  lifecycle {
    ignore_changes = [
      "metadata",
      "data"
    ]
  }
}

resource "kubernetes_ingress" "traefik_admin_ingress" {
  metadata {
    name      = "traefik-${var.ingress_class}-web-ui"
    namespace = "kube-system"

    annotations = {
      "certmanager.k8s.io/acme-challenge-type" = "http"
      "certmanager.k8s.io/cluster-issuer"      = "lets-encrypt-issuer-prod"
      "ingress.kubernetes.io/protocol"         = "http"
      "kubernetes.io/ingress.class"            = var.ingress_class
    }
  }

  spec {
    rule {
      host = "traefik-admin.${var.domain}"

      http {
        path {
          backend {
            service_name = "traefik-${var.ingress_class}-web-ui"
            service_port = "80"
          }
        }
      }
    }

    tls {
      hosts = [
        "traefik-admin.${var.domain}",
      ]

      secret_name = "traefik-${var.ingress_class}-cert"
    }
  }

  depends_on = [kubernetes_secret.admin_secret]
}

