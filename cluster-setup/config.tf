terraform {
  backend "remote" {
    hostname     = "app.terraform.io"
    organization = "rsmachiner"

    workspaces {
      name = "cluster-setup"
    }
  }
}

provider "kubernetes" {
}
