# Anthill - An operator for Gluster clusters

**Anthill** is an operator for managing Gluster clusters serving storage for
Kubernetes clusters. An operator is a Kubernetes
[controller](https://github.com/kubernetes/sample-controller) that is focused on
managing a specific application with the goal of automating common tasks that an
administrator would typically perform. Anthill can manage multiple Gluster clusters within a single Kubernetes installation, and those clusters need not be homogeneous in either configuration or topology.

## Building

While the current repository resides under the `jarrpa` namespace, it is
intended to eventually move to the `gluster` organization namespace. The project
is currently written in anticipation of that move, hence it is not possible to
simply perform a `go get`.

For some directory `<src>` in your `GOPATH`:

```
mkdir -p <src>/github.com/gluster
cd <src>/github.com/gluster
git checkout git@github.com:jarrpa/anthill.git
cd anthill
dep ensure
make
```

The Makefile recognizes several environment variables

* `PROJECT`: Project name. This is used as part of the path inside the build
  container as well as the name of the container image.

  **Default:** `gluster/anthill`

* `VERSION`: The container image version.

  **Default:** `latest`

* `IMAGE`: The full image name.

  **Default:** `$(PROJECT):$(VERSION)`

* `CONTAINER_DIR`: The directory inside the container where the project will be
  recognized in the GOPATH.

  **Default:** `/go/src/github.com/$(PROJECT)`

* `DOCKER_CMD`: The Docker command to use on the host system.

  **Default:** `docker`

**NOTE:** The build process uses Docker, so make sure you either run as root or have sudo access to run `docker`. The following may be a useful shortcut:

```
DOCKER_CMD="sudo /usr/bin/docker" make
```

## Deployment

See the sample YAML manifests in the [hack/deploy](./hack/deploy) directory.

## Contact

**Project Lead/Maintainer:** Jose A. Rivera - [@jarrpa](https://github.com/jarrpa)

The Anthill developers hang out in #sig-storage on the Kubernetes Slack and in
the #gluster IRC channel on the freenode network.

And, of course, Issues and Pull Requests are always welcomed.

## Project Management

For a glimpse into the broader state of things, visit our [Trello
board](https://trello.com/b/EvlcSiGc/anthill-development).
