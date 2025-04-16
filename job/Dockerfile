FROM debian:12.7-slim

LABEL maintainer=""

RUN set -x \
        && apt-get update \
        && apt-get install --no-install-recommends --no-install-suggests -y  apt-transport-https ca-certificates curl gnupg openssl gettext-base \
	&& curl -k -L https://artifactorycn.netcracker.com/nc.thirdparty.files/kubernetes/kubectl/v1.30.3/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
	&& chmod 0755 /usr/local/bin/kubectl \
	&& apt-get clean

COPY	--chmod=755 script.sh /

RUN useradd -ms /bin/bash -u 1001 job

USER 1001
