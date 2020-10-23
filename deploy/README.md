# Overview

This directory contains the YAML manifests and Kustomization resources necessary to deploy the runner application in different environments and cluster types.

## Base Directory

The base directory contains a list of shared YAML manifests that are needed to deploy the runner application.

## Overlays Directory

The overlays directory contains a list of additional resources and modifications (i.e. different namespace, image tag, etc.) needed to deploy the runner or any additional applications on differing environments or cluster types.

## TODO

- Need to establish multiple bases
- Need to establish multiple overlays depending on the cluster platform and environment
