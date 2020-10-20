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

if ! oc -n ${NAMESPACE} get deployment postgres >/dev/null 2>&1; then
    oc -n ${NAMESPACE} create -f ${MANIFEST_DIR}/db/deployment.yaml
fi

if ! oc -n ${NAMESPACE} get service postgres >/dev/null 2>&1; then
    oc -n ${NAMESPACE} create -f ${MANIFEST_DIR}/db/service.yaml
fi

if ! oc -n ${NAMESPACE} get cronjob exec-runner >/dev/null 2>&1; then
    oc -n ${NAMESPACE} create -f ${MANIFEST_DIR}/runner/job.yaml
fi

if ! oc -n ${NAMESPACE} get serviceaccount runner >/dev/null 2>&1; then
    oc -n ${NAMESPACE} create -f ${MANIFEST_DIR}/runner/sa.yaml
fi

if ! oc get clusterrole --show-labels | grep "app=runner" | awk '{ print $1 }' | xargs oc get clusterrole >/dev/null 2>&1; then
    oc create -f ${MANIFEST_DIR}/runner/cluster-role.yaml
fi

if ! oc get clusterrolebinding --show-labels | grep "app=runner" | awk '{ print $1 }' | xargs oc get clusterrole >/dev/null 2>&1; then
    oc create -f ${MANIFEST_DIR}/runner/cluster-role-binding.yaml
fi
