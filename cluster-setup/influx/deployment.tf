resource "kubernetes_deployment" "influx_deployment" {
  metadata {
    name      = "influxdb"
    namespace = var.namespace
  }
  spec {
    replicas = 1
    selector {
      match_labels = {
        k8s-app = "influxdb"
      }
    }
    template {
      metadata {
        labels = {
          task    = "metrics"
          k8s-app = "influxdb"
        }
      }

      spec {
        container {
          name  = "influxdb"
          image = "arm64v8/influxdb:1.7"
          volume_mount {
            mount_path = "/data"
            name       = "influxdb-storage"
          }
        }
        volume {
          name = "influxdb-storage"
        }
      }
    }
  }
}

resource "kubernetes_service" "influx_service" {
  metadata {
    name = "influx"
  }
  spec {
    selector = {
      app = "${kubernetes_deployment.influx_deployment.metadata.0.name}"
    }
    port {
      port        = 8086
      target_port = 8086
    }

    type = "ClusterIP"
  }
}
