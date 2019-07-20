# Cluster Setup (WIP)
`IMPORTANT: This was built for ARM64, other architectures are WIP`
Collection of Terraform Files that can be applied to your cluster to setup the following components:

* NATS Operator
* NATS Streaming Operator
* NATS Cluster
* NATS Streaming Cluster
* Optional Components
  * Traefik
  * Metallb
  * CertManager

### Requirements
* Kubernetes Cluster 1.13+
* Terraform Setup(app.terraform.io)

### TODO:
* Convert most YAML to Terraform and add
