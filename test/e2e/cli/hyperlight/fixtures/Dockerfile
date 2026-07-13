FROM golang:1.21.3-bookworm AS build

RUN apt-get update && apt-get install -y --no-install-recommends python3 && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY ./main.go /src/main.go

RUN set -xe; \
    CGO_ENABLED=0 \
    GOARCH=amd64 \
    go build \
      -buildmode=pie \
      -ldflags="-linkmode=internal" \
      -o /server main.go \
    ;

COPY patch_interp.py /patch_interp.py
RUN python3 /patch_interp.py /server

FROM scratch AS rootfs

COPY --from=build /server /bin/server
