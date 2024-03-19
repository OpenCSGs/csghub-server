FROM golang:1.21.0-bullseye
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /starhub
COPY . .
RUN  CGO_ENABLED=1 GOOS=linux go build  -v -o /go/bin/starhub ./cmd/csghub-server && \
     rm -rf /go/pkg && \
     rm -rf /starhub

WORKDIR /starhub-bin
ENV GIN_MODE=release
RUN cp  /go/bin/starhub . && \
    wget https://opencsg-public-resource.oss-cn-beijing.aliyuncs.com/tools/jq-linux-amd64 -O /usr/local/bin/jq && \
    chmod 755 /usr/local/bin/jq
COPY scripts/init.sh /starhub-bin/scripts/
COPY builder/store/database/seeds/. /starhub-bin/builder/store/database/seeds/
RUN chmod +x /starhub-bin/scripts/init.sh

RUN sed -i "s@/deb.debian.org@/mirrors.cloud.aliyuncs.com@g" /etc/apt/sources.list  && \
    apt-get update  && apt-get install -y cron && rm -rf /var/lib/apt/lists/*
EXPOSE 8080
ENTRYPOINT ["/starhub-bin/scripts/init.sh"]
