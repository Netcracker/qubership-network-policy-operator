#!/bin/bash

create_secret() {
        openssl genrsa -out ./tls.key 4096 \
		&& echo "$(date +%FT%H:%M:%S%:z) The private key has been created"

	echo "
[ req ]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn

[ dn ]
CN = ${SECRET}
O = remote-clusters

[ v3_ext ]
authorityKeyIdentifier=keyid,issuer:always
basicConstraints=CA:FALSE
keyUsage=keyEncipherment,dataEncipherment
extendedKeyUsage=clientAuth" > ./csr.cnf
	openssl req -config ./csr.cnf -new -key ./tls.key -nodes -out tls.csr
	REQUEST=$(cat ./tls.csr | base64 | tr -d '\n')
echo "---
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: ${SECRET}-csr
spec:
  groups:
    - system:authenticated
  request: ${REQUEST} 
  signerName: kubernetes.io/kube-apiserver-client
  username: ${SECRET}
  usages:
    - digital signature
    - key encipherment
    - client auth" > ./csr.yaml
	cat ./csr.yaml | envsubst | kubectl apply -f - > /dev/null 2>&1 \
		&& echo "$(date +%FT%H:%M:%S%:z) The CertificateSigningRequest has been applied"

	kubectl certificate approve ${SECRET}-csr > /dev/null 2>&1 \
		&& echo "$(date +%FT%H:%M:%S%:z) The Certificate has been approved"
	while [ ! -s tls.crt ]
	do
		sleep 3
		echo "$(date +%FT%H:%M:%S%:z) Retrying to get the Certificate"
		kubectl get csr ${SECRET}-csr -o jsonpath='{.status.certificate}' | base64 -d > ./tls.crt
	done
	kubectl delete csr ${SECRET}-csr > /dev/null 2>&1

	kubectl -n ${POD_NAMESPACE} get cm kube-root-ca.crt -o jsonpath='{.data.ca\.crt}' > ./ca.crt \
		&& echo "$(date +%FT%H:%M:%S%:z) The CA Certificate has been copied"

        kubectl -n ${POD_NAMESPACE} create secret generic ${SECRET} \
		--from-file=tls.crt=./tls.crt \
		--from-file=tls.key=./tls.key \
		--from-file=ca.crt=./ca.crt > /dev/null 2>&1 \
		&& echo "$(date +%FT%H:%M:%S%:z) The TLS Secret has been created"
}

cd /tmp

echo "$(date +%FT%H:%M:%S%:z) The Job has been run in ${POD_NAMESPACE} Namespace"

TOKEN=$(cat /run/secrets/kubernetes.io/serviceaccount/token)

echo "$(date +%FT%H:%M:%S%:z) Creating kubeconfig file"

cp /tmp/cm/kubeconfig ./kubeconfig

sed -i "s/\(token:\)/\1 ${TOKEN}/" ./kubeconfig

export KUBECONFIG=./kubeconfig

echo "$(date +%FT%H:%M:%S%:z) Checking the TLS Secret"

if !(kubectl -n ${POD_NAMESPACE} get secret ${SECRET} -o jsonpath='{.data.tls\.crt}' 2> /dev/null | base64 -d | openssl x509 -noout -checkend 1209600 > /dev/null 2>&1)
then
	echo "$(date +%FT%H:%M:%S%:z) The TLS Secret should be created or renewed"
	kubectl -n ${POD_NAMESPACE} delete secret ${SECRET} > /dev/null 2>&1
	create_secret
else
	echo "$(date +%FT%H:%M:%S%:z) The TLS Secret is acceptable"
fi

exit 0
