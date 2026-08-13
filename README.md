# DevBridge Go Workspace

This repository is a Go workspace. Each service is an independent module.

```text
relay-controller/   Relay Controller service
mgmt-service/        Namespace and API key management service
```

Run workspace checks from the repository root:1

```bash
go test ./relay-controller/...
go test ./mgmt-service/...
go vet ./relay-controller/...
go vet ./mgmt-service/...
```
