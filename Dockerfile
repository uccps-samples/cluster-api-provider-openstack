FROM registry.ci.openshift.org/openshift/release:golang-1.17 AS builder
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-openstack
COPY . .

RUN go build -o ./machine-controller-manager ./cmd/manager

FROM registry.ci.openshift.org/openshift/origin-v4.0:base

COPY --from=builder /go/src/sigs.k8s.io/cluster-api-provider-openstack/machine-controller-manager /
