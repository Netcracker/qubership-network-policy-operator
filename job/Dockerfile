FROM bash:5.3.9-alpine3.23

LABEL maintainer=""

RUN set -x \
        && apk add curl openssl envsubst \
	&& curl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl \
	&& chmod 0755 /usr/local/bin/kubectl \
	&& apk del curl

COPY	--chmod=755 job/tls.sh /

RUN adduser -h /home/job -s /bin/sh -u 1001 -D job

USER 1001
