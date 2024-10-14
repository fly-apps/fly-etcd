FROM golang:1.23

ENV ETCD_VERSION=v3.5.16
ARG FLY_VERSION=custom

ENV DOWNLOAD_URL=https://github.com/etcd-io/etcd/releases/download

WORKDIR /go/src/github.com/fly-apps/fly-etcd

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -v -o /fly/bin/start ./cmd/start
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /fly/bin/flycheck ./cmd/flycheck
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /fly/bin/flyadmin ./cmd/flyadmin

RUN curl -L ${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz \
 && tar xzvf /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz -C /usr/local/bin --strip-components=1

FROM alpine:3.3

ARG FLY_VERSION
ARG ETCD_VERSION

LABEL fly.app_role=etcd_cluster
LABEL fly.version=${FLY_VERSION}
LABEL fly.etcd-version=${ETCD_VERSION}

RUN apk add --update curl bash && \
   rm -rf /var/cache/apk/*

COPY --from=0 /usr/local/bin/etcd* /usr/local/bin
COPY --from=0 /fly/bin/* /usr/local/bin/

EXPOSE 2379

CMD ["start"]
