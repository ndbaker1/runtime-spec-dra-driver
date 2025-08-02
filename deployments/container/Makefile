include $(CURDIR)/common.mk

CONTAINER_TOOL ?= docker
BASE_IMAGE ?= docker.io/ubuntu:22.04
DOCKERFILE ?= $(CURDIR)/deployments/container/Dockerfile

### Deduced ###

IMAGE_TAG := $(VERSION)
IMAGE = $(IMAGE_NAME):$(IMAGE_TAG)

### Public ###

build:
	DOCKER_BUILDKIT=1 \
		$(CONTAINER_TOOL) build --pull \
		--tag $(IMAGE) \
		--build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
		--build-arg BASE_IMAGE="$(BASE_IMAGE)" \
		--build-arg VERSION="$(VERSION)" \
		-f $(DOCKERFILE) \
		$(CURDIR)

push:
	$(CONTAINER_TOOL) push $(IMAGE)
