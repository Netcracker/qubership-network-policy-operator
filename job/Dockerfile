FROM bash:devel-alpine3.22

LABEL maintainer=""

RUN set -x \
        && apk add curl openssl envsubst \
	&& curl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl \
	&& chmod 0755 /usr/local/bin/kubectl

COPY	--chmod=755 job/script.sh /

RUN adduser -h /home/job -s /bin/sh -u 1001 -D job

USER 1001
