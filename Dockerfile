FROM golang:1.14-stretch AS build-env
RUN mkdir -p /go/src/k8s-admission-controler
WORKDIR /go/src/k8s-admission-controler
COPY  . .
RUN useradd -u 10001 webhook
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o k8s-ac

FROM scratch
COPY --from=build-env /go/src/k8s-admission-controler/k8s-ac .
COPY --from=build-env /etc/passwd /etc/passwd
USER webhook
ENTRYPOINT ["/k8s-ac"]