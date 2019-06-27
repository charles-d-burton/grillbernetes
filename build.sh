#!/bin/bash

export DOCKER_CLI_EXPERIMENTAL=enabled

#Compile every architecture
arches=("amd64" "arm" "arm64")
repos=("charlesdburton/grillbernetes-events")
for arch in "${arches[@]}"
do
  docker build -f events/Dockerfile --build-arg GOARCH=$arch -t ${repos[0]}:${arch} .
  repos+=("${repos[0]}:${arch}")
done

#Login to Docker and push the images
docker login
for image in "${repos[@]:1}"
do
  echo "pushing: $image"
  docker push $image
done

#Create the docker manifest for all of the images
echo "Creating manifest for ${repos[@]}"
docker manifest create --amend "${repos[@]}"

#Tag every image with the correct architecture
for arch in "${arches[@]}"
do
echo "Annotating ${repos[0]}:${arch}"
docker manifest annotate --arch ${arch} ${repos[0]} ${repos[0]}:${arch}
done

docker manifest push charlesdburton/grillbernetes-events
