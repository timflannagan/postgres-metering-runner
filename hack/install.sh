#! /bin/bash

set -oeu pipefail

namespace=${1:-tflannag}

if ! oc get namespace ${NAMESPACE} >/dev/null 2>&1; then
    oc create namespace ${NAMESPACE}
fi

oc adm policy add-scc-to-user -z default anyuid

if ! oc -n ${NAMESPACE} get deployment postgres 2> /dev/null; then
    oc -n ${NAMESPACE} create -f manifests/db/deployment.yaml
fi

if ! oc -n ${NAMESPACE} get service postgres 2> /dev/null; then
    oc -n ${NAMESPACE} create -f manifests/db/service.yaml
fi
