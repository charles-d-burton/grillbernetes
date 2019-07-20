module "traefik" {
  source = "./traefik"
  providers = {
    kubernetes = "kubernetes"
  }
}
