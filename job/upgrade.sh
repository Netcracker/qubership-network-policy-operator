#!/usr/bin/env bash

cd /tmp

echo "$(date +%FT%H:%M:%S%:z) The Job has been run in ${POD_NAMESPACE} Namespace"

TOKEN=$(cat /run/secrets/kubernetes.io/serviceaccount/token)

echo "$(date +%FT%H:%M:%S%:z) Creating local kubeconfig file"

cp /tmp/cm/kubeconfig ./kubeconfig

sed -i "s/\(token:\)/\1 ${TOKEN}/" ./kubeconfig

export KUBECONFIG=./kubeconfig

kubectl -n ${POD_NAMESPACE} scale deployment ${DEPLOYMENT_NAME} --replicas=0
echo "$(date +%FT%H:%M:%S%:z) The ${DEPLOYMENT_NAME} deployment has been scaled down to 0"

LIST=$(mktemp -p ./)
kubectl -n ${POD_NAMESPACE} get rclc --no-headers=true | awk '{ print $1}' > $LIST

while read STR
do
	kubectl -n ${POD_NAMESPACE} patch rclc ${STR} --type json -p '[{"op": "remove", "path": "/metadata/finalizers"}]'
	echo "$(date +%FT%H:%M:%S%:z) The ${STR} RemoteClusterConfig has been patched"

done < $LIST

echo "$(date +%FT%H:%M:%S%:z) The Job is done"

exit 0
