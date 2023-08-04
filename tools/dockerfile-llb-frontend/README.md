# KraftKit LLB Plugin

This KraftKit LLB Plugin enables running Unikraft builds using Docker commands.

It eliminates the need for installing KraftKit or Unikraft-specific dependencies on your machine by executing builds in containers.

The result is a Unikraft image saved in your local Docker registry.

**Please note:** If you are unfamiliar with Unikraft, KraftKit, or unikernels, kindly refer to the [Unikraft Documentation](https://unikraft.org/docs/getting-started/).

## Prerequisites

- Docker with BuildKit enabled (this is the default setting from Docker v23.0 onwards)
- Go v1.20 or later

## Usage

There are two ways to use the LLB plugin:

### 1. Docker-based Usage

Build the Docker image containing the plugin:

```sh
docker build . -t kraftkit.sh/llb
```

Modify your Kraftfile file by adding the following line at the top:

```yaml
#syntax=kraftkit.sh/llb:latest
```

Now, run the Docker build:

```sh
docker build -f test/apps/app-helloworld/kraft.yaml test/apps/app-helloworld
```

See [Docker docs about dynamic frontends](https://docs.docker.com/build/dockerfile/frontend/) for more information.

### 2. Direct Binary Usage (Debug Mode)

In this mode, you can output the LLB graph for inspection with buildkit.

First, build the project:

```sh
go build .
```

Then, run the built binary from within a Unikraft app directory:

```sh
dockerfile-llb-frontend --llb-stdout=true | buildctl debug dump-llb
```

To learn more about this, see this [BuildKit doc](https://github.com/moby/buildkit/blob/master/examples/README.md).

## Contributing

We warmly welcome contributions in the form of tests, bug reports, and feature requests.

For discussions or queries, join us on Discord: https://bit.ly/UnikraftDiscord

### Testing

The project has unit tests for the core build part (./build/build_test.go) and end-to-end
tests suites in the test/apps directory, one for BuildKit and one for Docker.

The Docker suite calls the docker-backing BuildKit daemon so in a way the BuildKit tests
are redundant, thus we omit them from the CI flow. They come in handy if you want to
run a BuildKit daemon with a debugger.

To run these tests:

Docker:

```sh
go test ./test/docker -v
```

BuildKit (you have to run buildkit daemon):

To spawn a BuildKit daemon, run:

```sh
buildkitd
```

Then run the test suite:

```sh
go test ./test/buildkit -v
```

The -v option gives you the output of the BuildKit or Docker runs.
