PROJECT = gluster/anthill
CONTAINER_DIR = /go/src/github.com/$(PROJECT)

ifeq ($(VERSION),)
  VERSION = latest
endif
ifeq ($(IMAGE),)
  IMAGE = $(PROJECT):$(VERSION)
endif
ifeq ($(DOCKER_CMD),)
  DOCKER_CMD = docker
endif


.PHONY: all build container push test clean

all: dep codegen build

codegen:
	rm -rf pkg/client
	./hack/update-codegen.sh

dep:
	dep ensure

build:
	@mkdir -p _output/bin
	$(DOCKER_CMD) run --rm \
		-e "CGO_ENABLED=0" \
		-e "GOCACHE=off" \
		-u $$(id -u):$$(id -g) \
		-v $$(pwd):$(CONTAINER_DIR) \
		-w $(CONTAINER_DIR) \
		golang:1.10-alpine \
		go build -installsuffix "anthill" -o _output/bin/anthill ./cmd/anthill

quick-container:
	$(DOCKER_CMD) build -t $(IMAGE) .

container: build quick-container

push: container
	$(DOCKER_CMD) push $(IMAGE)

full: dep codegen push

test:
	go test `go list ./... | grep -v 'vendor'`

clean:
	rm -rf _output
