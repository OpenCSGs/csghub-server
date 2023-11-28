FROM golang:latest as builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /starhub
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -tags netgo -a -v -o starhub ./cmd/starhub-server


FROM alpine:latest as prod
WORKDIR /starhub-bin
ENV GIN_MODE=release
COPY --from=0 /starhub/starhub .
RUN apk add --no-cache --upgrade bash curl jq
COPY scripts/init.sh /starhub-bin/scripts/
RUN chmod +x /starhub-bin/scripts/init.sh
EXPOSE 8080
ENTRYPOINT ["/starhub-bin/scripts/init.sh"]
