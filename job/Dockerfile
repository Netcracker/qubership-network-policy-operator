FROM ghcr.io/netcracker/qubership-docker-kubectl:0.0.8

LABEL maintainer=""

USER 0

RUN set -x \
        && apk add --update --upgrade --no-cache openssl envsubst 

RUN set -x \
    && wget \
        --no-check-certificate \
        -nv \
        -O /usr/bin/kubectl \
        "https://dl.k8s.io/v1.36.2/bin/linux/${TARGETARCH}/kubectl" \
    && chmod +x /usr/bin/kubectl

USER 1001
