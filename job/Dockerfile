FROM debian:12.11-slim

LABEL maintainer=""

RUN set -x \
        && apt-get update \
        && apt-get install --no-install-recommends --no-install-suggests -y  apt-transport-https ca-certificates curl gnupg openssl gettext-base \
	&& curl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl \
	&& chmod 0755 /usr/local/bin/kubectl \
	&& apt-get clean

COPY	--chmod=755 job/script.sh /

RUN useradd -ms /bin/bash -u 1001 job

USER 1001
