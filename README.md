# Grillbernetes

A collection of software and infrastructure to manage a Smoker or other temperature managed device with Kubernetes.  Keep in mind that this was designed to work on a K8S cluster that was local to the network.  I would appreciate some help getting SSL in order on STAN to that it could be connected to any K8S cluster.  Keep in mind that some knowledge of K8S is necessary to get this to work.

### Requirements
* Go 1.12+
* Kubernetes 1.14+
* A working Kubernetes Cluster with the following:
  * Certmanager
  * A forwarded Service and Ingress
* Raspberry Pi(s) or other SBC
* DS18B20 Sensors
* Relays
* Some device that needs relatively precise temperature control
* (Other things that I've likely forgotten)

### Building the components(Other than the PiSmoker)
Run:
```bash
$./build.sh -u <your dockerhub username>
```

This will build and upload the artifacts to your personal Docker Hub compiled for
* ARM
* ARM64
* AMD64

Projects are configured to pull in the correct image based on your SBC arch.

## Components By Directory
Review the README for each piece for technical details.
### Cluster Setup
Collection of Kubernetes/Terraform Files that setup the base layer necessary.  It will install the NATS and NATS Streaming operators, services, and deployments to coalesce a fully functioning NATS Streaming Cluster.  Optionally there is code in there to setup Traefik as an Ingress to your cluster.  Read the README in that directory for more information.

### Control Hub
The program and K8S installation files to build/deploy the control mechanism.  This allows you to take control of a device and turn it on/off as well as set the desired temperature state.


### Events
Consumes the event stream from NATS Streaming and publishes it to the `/events/` path using Server Side Events.  Is literally just an event stream, more to come for multi-device control.

### Frontend
WIP


### PiSmoker
Program to interface and control a Smoker attached to a Raspberry Pi.

### Vue-Frontend
SPA(WIP)

### Global TODO:
* Align topic handling to be configurable in all services
* Allow event publisher to serve multiple topics
* Set control hub to allow control of multiple topics
* (Maybe those should be kubeless?)