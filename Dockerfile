# This Dockerfile contains multiple targets.
# Use 'docker build --target=<name> .' to build one.

# ===================================
#   Non-release images.
# ===================================

# devbuild compiles the binary
# -----------------------------------
FROM golang:1.17 AS devbuild

# Disable CGO to make sure we build static binaries
ENV CGO_ENABLED=0

# Escape the GOPATH
WORKDIR /build
COPY . ./
RUN go build -o nomad-pack .

# dev runs the binary from devbuild
# -----------------------------------
FROM alpine:3.15 AS dev

RUN apk add --no-cache git libc6-compat
COPY --from=devbuild /build/nomad-pack /bin/
COPY ./scripts/docker-entrypoint.sh /

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["help"]

# ===================================
#   Release images.
# ===================================
FROM alpine:3.15 AS release

ARG PRODUCT_NAME=nomad-pack
ARG PRODUCT_VERSION
ARG PRODUCT_REVISION
# TARGETARCH and TARGETOS are set automatically when --platform is provided.
ARG TARGETOS TARGETARCH

LABEL maintainer="Nomad Team <nomad@hashicorp.com>"
LABEL version=${PRODUCT_VERSION}
LABEL revision=${PRODUCT_REVISION}

RUN apk add --no-cache git libc6-compat

COPY dist/$TARGETOS/$TARGETARCH/nomad-pack /bin/
COPY ./scripts/docker-entrypoint.sh /

# Create a non-root user to run the software.
RUN addgroup $PRODUCT_NAME && \
    adduser -S -G $PRODUCT_NAME $PRODUCT_NAME && \
    mkdir -p /home/$PRODUCT_NAME/.cache && \
    chown $PRODUCT_NAME /home/$PRODUCT_NAME/.cache

USER $PRODUCT_NAME
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["help"]

# ===================================
#   Set default target to 'dev'.
# ===================================
FROM dev
