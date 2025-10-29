#!/usr/bin/env bash

# Export Environment
export BUILDS_DIR=${BUILDS_DIR:-builds}
export CONTAINER_TOOL=${CONTAINER_TOOL:- docker}
export DOCKER_REPO=${DOCKER_REPO:-"quay.io/openshift-pipeline"}
export PLATFORMS=${PLATFORMS:-"linux/amd64"}


# get version details
export BUILD_DATE=`date -u +'%Y-%m-%dT%H:%M:%S%:z'`
export GIT_BRANCH=`git rev-parse --abbrev-ref HEAD`
export GIT_SHA=`git rev-parse HEAD`
export GIT_SHA_SHORT=`git rev-parse --short HEAD`


mkdir -p ${BUILDS_DIR}
rm  -rf ${BUILDS_DIR}/*

# update tag, if available
if [ ${GIT_BRANCH} = "HEAD" ]; then
  export GIT_BRANCH=`git describe --abbrev=0 --tags`
fi

# update version number
export VERSION=`echo ${GIT_BRANCH} |  awk 'match($0, /([0-9]*\.[0-9]*\.[0-9]*)$/) { print "v" substr($0, RSTART, RLENGTH) }'`
if [ -z "$VERSION" ]; then
  export VERSION="devel-0"
fi

# 👇 Export to GitHub Actions environment if running in CI
if [ -n "$GITHUB_ENV" ]; then
  echo "VERSION=$VERSION" >> "$GITHUB_ENV"
fi

export IMG=$DOCKER_REPO:$VERSION
echo $IMG

#make manifests kustomize  docker-buildx
(cd config/manager && kustomize edit set image controller=${IMG})
(cd config/webhook && kustomize edit set image controller=${IMG})

kustomize build config/default -o ${BUILDS_DIR}/release-${VERSION}.yaml

cd  ${BUILDS_DIR}
echo $BUILD_DATE > build_timestamp.txt
sha256sum *.yaml > SHA256SUMS.txt

