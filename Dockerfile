FROM openshift/golang-builder@sha256:4820580c3368f320581eb9e32cf97aeec179a86c5749753a14ed76410a293d83 AS builder
ENV __doozer=update BUILD_RELEASE=202202160023.p0.ge6b35eb.assembly.stream BUILD_VERSION=v4.10.0 OS_GIT_MAJOR=4 OS_GIT_MINOR=10 OS_GIT_PATCH=0 OS_GIT_TREE_STATE=clean OS_GIT_VERSION=4.10.0-202202160023.p0.ge6b35eb.assembly.stream SOURCE_GIT_TREE_STATE=clean 
ENV __doozer=merge OS_GIT_COMMIT=e6b35eb OS_GIT_VERSION=4.10.0-202202160023.p0.ge6b35eb.assembly.stream-e6b35eb SOURCE_DATE_EPOCH=1642014618 SOURCE_GIT_COMMIT=e6b35eb43631023a38b92460661130531236347c SOURCE_GIT_TAG=e6b35eb4 SOURCE_GIT_URL=https://github.com/uccps-samples/cluster-api-provider-openstack 
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-openstack
COPY . .

RUN go build -o ./machine-controller-manager ./cmd/manager

FROM openshift/ose-base:v4.10.0.20220216.010142
ENV __doozer=update BUILD_RELEASE=202202160023.p0.ge6b35eb.assembly.stream BUILD_VERSION=v4.10.0 OS_GIT_MAJOR=4 OS_GIT_MINOR=10 OS_GIT_PATCH=0 OS_GIT_TREE_STATE=clean OS_GIT_VERSION=4.10.0-202202160023.p0.ge6b35eb.assembly.stream SOURCE_GIT_TREE_STATE=clean 
ENV __doozer=merge OS_GIT_COMMIT=e6b35eb OS_GIT_VERSION=4.10.0-202202160023.p0.ge6b35eb.assembly.stream-e6b35eb SOURCE_DATE_EPOCH=1642014618 SOURCE_GIT_COMMIT=e6b35eb43631023a38b92460661130531236347c SOURCE_GIT_TAG=e6b35eb4 SOURCE_GIT_URL=https://github.com/uccps-samples/cluster-api-provider-openstack 

COPY --from=builder /go/src/sigs.k8s.io/cluster-api-provider-openstack/machine-controller-manager /

LABEL \
        name="openshift/ose-openstack-machine-controllers" \
        com.redhat.component="ose-openstack-machine-controllers-container" \
        io.openshift.maintainer.product="OpenShift Container Platform" \
        io.openshift.maintainer.component="Cloud Compute" \
        io.openshift.maintainer.subcomponent="OpenStack Provider" \
        release="202202160023.p0.ge6b35eb.assembly.stream" \
        io.openshift.build.commit.id="e6b35eb43631023a38b92460661130531236347c" \
        io.openshift.build.source-location="https://github.com/uccps-samples/cluster-api-provider-openstack" \
        io.openshift.build.commit.url="https://github.com/uccps-samples/cluster-api-provider-openstack/commit/e6b35eb43631023a38b92460661130531236347c" \
        version="v4.10.0"

