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

COPY bin/fusion fusion
COPY script.mjs script.mjs

ENTRYPOINT fusion