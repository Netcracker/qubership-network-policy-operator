FROM ghcr.io/netcracker/qubership-docker-kubectl:0.0.11

LABEL maintainer=""

USER 0

RUN set -x \
        && apk add --update --upgrade --no-cache openssl envsubst 

USER 1001
