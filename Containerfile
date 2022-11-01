FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y \
        curl \
        dnsutils \
        nodejs \
    && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -ms /bin/bash main
USER main
WORKDIR /home/main

VOLUME /tmp/fusion

RUN mkdir -p secrets
VOLUME secrets/tls

COPY development/paseto.pub secrets/paseto.pub
COPY bin/fusion fusion

ENTRYPOINT fusion
