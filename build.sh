#!/bin/bash
#Login to Docker and push the images

export DOCKER_CLI_EXPERIMENTAL=enabled

builds=("events" "control-hub")
arches=("amd64" "arm" "arm64")
echo "Building ${builds[@]}"
for dir in "${builds[@]}"
do
  echo "Building ${dir} now"
  #Compile every architecture
  cd ${dir}
  repos=("charlesdburton/grillbernetes-${dir}")
  for arch in "${arches[@]}"
  do
    docker build -f Dockerfile --build-arg GOARCH=$arch -t ${repos[0]}:${arch} .
    repos+=("${repos[0]}:${arch}")
  done

  
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

  echo "Pushing manifest"
  docker manifest push charlesdburton/grillbernetes-${dir}
  cd ..
done
