FROM node:18

# 安裝 Go
ENV GOLANG_VERSION=1.21.3

RUN curl -LO https://golang.org/dl/go${GOLANG_VERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GOLANG_VERSION}.linux-amd64.tar.gz && \
    rm go${GOLANG_VERSION}.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

RUN apt-get update && apt-get install -y \
    git \
    curl \
    bash \
    vim \
    build-essential

WORKDIR /workspace
