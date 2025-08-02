GOLANG_VERSION ?= 1.24.4

DRIVER_NAME ?= resource-spec-dra-driver

VERSION  ?= v0.1.0
vVERSION := v$(VERSION:v%=%)

VENDOR ?= example.com
APIS ?= v1alpha1

ifeq ($(IMAGE_NAME),)
REGISTRY ?= registry.example.com
IMAGE_NAME = $(REGISTRY)/$(DRIVER_NAME)
endif
