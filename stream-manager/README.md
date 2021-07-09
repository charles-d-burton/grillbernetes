# Stream Manager

Simple Kubernetes job to interface with a nats server and create Jetstreams.
This removes the need for each service to manage and create their streams
and manages the dependency of ensuring that all streams necessary for the
system to run have been created.  This could also be extended to other message
busses, but for now it only supports NATS Jetstream.

### TODO:
* Create tunables so we can reconfigure it as needed in the streamconfig
* Support additional message busses