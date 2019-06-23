#!/bin/bash

export DOCKER_CLI_EXPERIMENTAL=enabled

arches=("amd64" "arm" "arm64")
repos=("charlesdburton/grillbernetes-frontend")
for arch in "${arches[@]}"
do
  docker build -f events/Dockerfile --build-arg GOARCH=$arch -t ${repos[0]}:${arch} .
  repos+=("${repos[0]}:${arch}")
done

#docker build -f events/Dockerfile-amd64 -t charlesdburton/grillbernetes-frontend:amd64 .
#docker build -f events/Dockerfile-arm64 -t charlesdburton/grillbernetes-frontend:arm64 .
#docker build -f events/Dockerfile-armhf -t charlesdburton/grillbernetes-frontend:armhf .
docker login
for image in "${repos[@]:1}"
do
  echo "pushing: $image"
  docker push $image
done

echo "Creating manifest for ${repos[@]}"
docker manifest create --amend "${repos[@]}"
#docker manifest create --amend charlesdburton/grillbernetes-frontend \
 #charlesdburton/grillbernetes-frontend:amd64 \
 #charlesdburton/grillbernetes-frontend:arm64 \
 #charlesdburton/grillbernetes-frontend:armhf

for arch in "${arches[@]}"
do
echo "Annotating ${repos[0]}:${arch}"
docker manifest annotate --arch ${arch} ${repos[0]} ${repos[0]}:${arch}
done
#docker manifest annotate --arch arm charlesdburton/grillbernetes-frontend charlesdburton/grillbernetes-frontend:armhf
#docker manifest annotate --arch arm64 charlesdburton/grillbernetes-frontend charlesdburton/grillbernetes-frontend:arm64
#docker manifest annotate --arch amd64 charlesdburton/grillbernetes-frontend charlesdburton/grillbernetes-frontend:amd64
docker manifest push charlesdburton/grillbernetes-frontend
