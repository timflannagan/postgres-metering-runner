FROM openshift/origin-release:golang-1.15 AS builder

WORKDIR /scratch
COPY ./pkg ./pkg
COPY ./cmd ./cmd
COPY ./go.mod ./go.mod
COPY ./go.sum ./go.sum
COPY ./vendor ./vendor
COPY Makefile ./

RUN make driver

FROM centos:8
USER 3001
COPY --from=builder /scratch/bin/driver /
ENTRYPOINT ["/scratch"]
