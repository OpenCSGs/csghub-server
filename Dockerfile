FROM golang:latest
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /starhub-server
COPY . .
RUN go build -o ./bin/starhub-server ./cmd/starhub-server
ENV GIN_MODE=release
EXPOSE 8080
ENTRYPOINT [ "/starhub-server/bin/starhub-server", "start", "server" ]
 