#!/bin/bash
#Login to Docker and push the images

export DOCKER_CLI_EXPERIMENTAL=enabled
sudo apt update && sudo apt install -y parallel
export builds=("events" "control-hub")
echo "Building ${builds[@]}"

function buildServices() {
  arches=("amd64" "arm" "arm64")
  dir=$1
  export repo="charlesdburton/grillbernetes-${dir}"
  echo "Building ${dir} now"

  function dockerBuild() {
    arch=$1
    docker build -f Dockerfile --build-arg GOARCH=$arch -t ${repo}:${arch} .
    docker push ${repo}:${arch}
  }
  export -f dockerBuild
  parallel dockerBuild ::: ${arches[@]}
  #Compile every architecture
  cd ${dir}
  repos=("charlesdburton/grillbernetes-${dir}")
  for arch in "${arches[@]}"
  do
    repos+=("${repos[0]}:${arch}")
  done

  
  #for image in "${repos[@]:1}"
  #do
  #  echo "pushing: $image"
  #  docker push $image
  #done

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
}
export -f buildServices
parallel buildServices ::: ${builds[@]}
