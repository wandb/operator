# Kubernetes HTTP APIs for Testing

The `*.http` files are for use with JetBrains HTTP Client. It is included in Goland and is available as a 
separate CLI tool.

The `proxy-*` environments in the [environment file](./http-client.env.json) are intended for use with 
a kube proxy to hit unauthenticated K8S HTTP API. Start it with `kubectl proxy --port=8001` and take great
care with your current context!

## Contents

* [update-infra](./update-infra.http) has patches to enable/disable infra and retention settings.
