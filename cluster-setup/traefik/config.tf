

data "template_file" "traefik_config" {
  template = file("${path.module}/traefik.toml")

  vars = {
    ingress_class = var.ingress_class
  }
}

