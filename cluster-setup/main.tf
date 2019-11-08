module "traefik" {
  source = "./traefik"
  providers = {
    kubernetes = "kubernetes"
  }
}

module "influx" {
  source = "./influx"
  providers = {
    kubernetes = "kubernetes"
  }
}
