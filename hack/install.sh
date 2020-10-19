#! /bin/bash

set -eou pipefail

NAMESPACE=${1:-tflannag}

BASE_DIR=$(dirname "${BASH_SOURCE}")/..
MANIFEST_DIR=${BASE_DIR}/manifests

if ! oc get namespace ${NAMESPACE} >/dev/null 2>&1; then
    oc create namespace ${NAMESPACE}
fi

oc -n ${NAMESPACE} adm policy add-scc-to-user -z default anyuid

if ! oc -n openshift-monitoring get prometheusrules metering >/dev/null 2>&1; then
    oc create -f ${MANIFEST_DIR}/monitoring/recording-rules.yaml
fi

if ! oc -n ${NAMESPACE} get deployment postgres 2> /dev/null; then
    oc -n ${NAMESPACE} create -f ${MANIFEST_DIR}/db/deployment.yaml
fi

if ! oc -n ${NAMESPACE} get service postgres 2> /dev/null; then
    oc -n ${NAMESPACE} create -f ${MANIFEST_DIR}/db/service.yaml
fi

if ! oc -n ${NAMESPACE} get deployment runner 2> /dev/null; then
    oc -n ${NAMESPACE} create -f ${MANIFEST_DIR}/runner/deployment.yaml
fi

# TODO: currently a service object isn't needed as we're not exposing any ports
# and we don't need to route traffic to the runner Pod.
# if ! oc -n ${NAMESPACE} get svc runner 2> /dev/null; then
#     oc -n ${NAMESPACE} create -f ${MANIFEST_DIR}/runner/service.yaml
# fi

if ! oc -n ${NAMESPACE} get serviceaccount runner 2> /dev/null; then
    oc -n ${NAMESPACE} create -f ${MANIFEST_DIR}/runner/sa.yaml
fi

if ! oc get clusterrole -l app=runner 2> /dev/null; then
    oc create -f ${MANIFEST_DIR}/runner/cluster-role.yaml
fi

if ! oc get clusterrolebinding -l app=runner 2> /dev/null; then
    oc create -f ${MANIFEST_DIR}/runner/cluster-role-binding.yaml
fi
