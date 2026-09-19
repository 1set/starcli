FROM debian:bookworm-slim

# Build the static linux/amd64 binary first with `make build_linux`.
# Named time zones are embedded by main.go's time/tzdata import.
ENV TERM=xterm-256color
ENV HOME=/work

RUN set -eux; \
    apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir /work && chown 65532:65532 /work

COPY --chmod=755 starcli /usr/local/bin/starcli
USER 65532:65532
WORKDIR /work
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/starcli"]
