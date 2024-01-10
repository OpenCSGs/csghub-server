FROM golang:1.21.0 as builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /starhub
COPY . .
RUN  CGO_ENABLED=1 GOOS=linux go build -tags netgo -a  -installsuffix cgo -ldflags '-extldflags "-static"'  -v -o starhub ./cmd/starhub-server


FROM alpine:latest as prod
WORKDIR /starhub-bin
ENV GIN_MODE=release
COPY --from=0 /starhub/starhub .
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g'  /etc/apk/repositories
RUN apk add --no-cache  bash curl jq
COPY scripts/init.sh /starhub-bin/scripts/
COPY builder/store/database/seeds/. /starhub-bin/builder/store/database/seeds/
RUN chmod +x /starhub-bin/scripts/init.sh
EXPOSE 8080
ENTRYPOINT ["/starhub-bin/scripts/init.sh"]