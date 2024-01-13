FROM --platform=linux/amd64 golang:1.21-bookworm

ARG CONTAINERLAB_VERSION="0.48.*"

RUN set -x && apt-get update -y && DEBIAN_FRONTEND=noninteractive apt-get install -y \
            ca-certificates  \
            lsb-release \
            wget \
            jq \
            procps \
            curl \
            vim \
            inetutils-ping binutils \
            iproute2 \
            tcpdump \
            net-tools && \
    rm -rf /var/lib/apt/lists/*

RUN echo "deb [trusted=yes] https://apt.fury.io/netdevops/ /" | \
    tee -a /etc/apt/sources.list.d/netdevops.list

RUN curl -fsSL https://download.docker.com/linux/debian/gpg | \
    gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

RUN echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian \
    $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

RUN apt-get update && \
    apt-get install -yq --no-install-recommends \
            containerlab=${CONTAINERLAB_VERSION} \
            docker-ce \
            docker-ce-cli && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* /var/cache/apt/archive/*.deb

COPY .develop/daemon.json /etc/docker/daemon.json

WORKDIR /slurpeeth

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

ENTRYPOINT ["sleep", "infinity"]
