#!/usr/bin/env bash

cd /tmp

echo "$(date +%FT%H:%M:%S%:z) The Job has been run in ${POD_NAMESPACE} Namespace"

TOKEN=$(cat /run/secrets/kubernetes.io/serviceaccount/token)

echo "$(date +%FT%H:%M:%S%:z) Creating local kubeconfig file"

cp /tmp/cm/kubeconfig ./kubeconfig

sed -i "s/\(token:\)/\1 ${TOKEN}/" ./kubeconfig

export KUBECONFIG=./kubeconfig

LIST=$(mktemp -p ./)
kubectl -n ${POD_NAMESPACE} get rclc --no-headers=true | awk '{ print $1}' > $LIST

while read STR
do
	SC=$(kubectl -n ${POD_NAMESPACE} get rclc $STR -o jsonpath='{.spec.remoteCluster.remoteSecret}')
	echo "$(date +%FT%H:%M:%S%:z) The remote secret is $SC"
	CL=$(kubectl -n ${POD_NAMESPACE} get rclc $STR -o jsonpath='{.spec.remoteCluster.name}')
	echo "$(date +%FT%H:%M:%S%:z) The remote cluster is $CL"
	API="https://$(kubectl -n ${POD_NAMESPACE} get rclc $STR -o jsonpath='{.spec.remoteCluster.api}')"
	echo "$(date +%FT%H:%M:%S%:z) The remote cluster API is $API"
	TOKEN=$(kubectl -n ${POD_NAMESPACE} get secret $SC -o jsonpath='{.data.token}' | base64 -d)
	echo "$(date +%FT%H:%M:%S%:z) Creating remote kubeconfig file"
	kubectl -n ${POD_NAMESPACE} delete pod --all --token=${TOKEN} --server=${API} --insecure-skip-tls-verify=true > /dev/null 2>&1 && echo "$(date +%FT%H:%M:%S%:z) The Network Policy Operator in ${CL} has been restarted"
done < $LIST

echo "$(date +%FT%H:%M:%S%:z) The Job is done"

exit 0
