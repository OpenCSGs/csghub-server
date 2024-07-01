FROM opencsg-registry.cn-beijing.cr.aliyuncs.com/public/starhub-server:base-1.0 AS build-env
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /starhub
COPY . .
RUN  CGO_ENABLED=1 GOOS=linux go build  -v -o /go/bin/starhub ./cmd/csghub-server && \
     rm -rf /go/pkg && \
     rm -rf /starhub

FROM opencsg-registry.cn-beijing.cr.aliyuncs.com/public/starhub-server:base-1.0    
WORKDIR /starhub-bin
ENV GIN_MODE=release
COPY --from=build-env /go/bin/starhub .
COPY scripts/init.sh /starhub-bin/scripts/
COPY builder/store/database/seeds/. /starhub-bin/builder/store/database/seeds/
RUN chmod +x /starhub-bin/scripts/init.sh

EXPOSE 8080
ENTRYPOINT ["/starhub-bin/scripts/init.sh"]
