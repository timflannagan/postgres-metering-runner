FROM openshift/origin-release:golang-1.15 AS builder

WORKDIR /scratch
COPY ./pkg ./pkg
COPY ./cmd ./cmd
COPY ./go.mod ./go.mod
COPY ./go.sum ./go.sum
COPY ./vendor ./vendor

RUN CGO_ENABLED=0 go build --ldflags '-w' -o /go/scratch ./cmd/driver/

FROM centos:8
USER 3001
COPY --from=builder /go/scratch /
ENTRYPOINT ["/scratch"]
