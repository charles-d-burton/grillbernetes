#!/bin/bash

export DOCKER_CLI_EXPERIMENTAL=enabled

docker build -f events/Dockerfile-amd64 -t charlesdburton/grillbernetes-frontend:amd64 .
docker build -f events/Dockerfile-arm64 -t charlesdburton/grillbernetes-frontend:arm64 .
docker build -f events/Dockerfile-armhf -t charlesdburton/grillbernetes-frontend:armhf .
docker login
for image in `docker image list | grep grillbernetes | awk '{print $1}'`
do
  docker push $image
done

docker manifest create --amend charlesdburton/grillbernetes-frontend \
 charlesdburton/grillbernetes-frontend:amd64 \
 charlesdburton/grillbernetes-frontend:arm64 \
 charlesdburton/grillbernetes-frontend:armhf

docker manifest annotate --arch arm charlesdburton/grillbernetes-frontend charlesdburton/grillbernetes-frontend:armhf
docker manifest annotate --arch arm64 charlesdburton/grillbernetes-frontend charlesdburton/grillbernetes-frontend:arm64
docker manifest annotate --arch amd64 charlesdburton/grillbernetes-frontend charlesdburton/grillbernetes-frontend:amd64
docker manifest push charlesdburton/grillbernetes-frontend
