FROM golang:1.16

WORKDIR /go/src/github.com/fly-examples/fly-etcd
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -v -o /fly/bin/start ./cmd/start

ENV ETCD_VER=v3.5.0
ENV DOWNLOAD_URL=https://github.com/etcd-io/etcd/releases/download

RUN apt-get update && apt-get install --no-install-recommends -y \
    ca-certificates curl bash dnsutils vim-tiny procps jq \
    && apt autoremove -y

RUN curl -L ${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz \
 && tar xzvf /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz -C /usr/local/bin --strip-components=1

RUN useradd -ms /bin/bash etcd

# COPY /fly/bin/* /usr/local/bin/

EXPOSE 2379

CMD ["/fly/bin/start"]
