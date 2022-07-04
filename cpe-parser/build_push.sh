#########################################################
##
## Script to build and push CPE parser docker image
##
## * Need to set IMAGE_REGISTRY environment
##
## Image Tag:  ${IMAGE_REGISTRY}/cpe/parser:v${VERSION}
## default version is 0.0.1
##
##########################################################
if [ -z $IMAGE_REGISTRY ]; then
    echo "Please set environment IMAGE_REGISTRY"
    exit
fi

if [ -z $VERSION ]; then
    VERSION="0.0.1"
fi

IMAGE="${IMAGE_REGISTRY}/cpe/parser"
VERSION="v"${VERSION}

IMAGE_VERSION=$IMAGE:$VERSION
echo Build and Push CPE Parser Image
echo $IMAGE_VERSION
docker build -t $IMAGE_VERSION .
docker push $IMAGE_VERSION