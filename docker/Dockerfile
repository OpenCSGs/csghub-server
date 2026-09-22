FROM opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/csghub_server:base-3.0-build-go1.26.0 AS build-env
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /starhub
COPY . .

ARG CSGHUB_TAGS
ARG CI_COMMIT_SHORT_SHA
ARG IMAGE_TAG
RUN CGO_ENABLED=1 GOOS=linux go build \
      -tags="$CSGHUB_TAGS" \
      -ldflags "-X opencsg.com/csghub-server/version.GitRevision=${CI_COMMIT_SHORT_SHA} -X opencsg.com/csghub-server/version.StarhubAPIVersion=${IMAGE_TAG}" \
      -v -o /go/bin/starhub ./cmd/csghub-server
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -v -o /go/bin/csghub_aliyun_checker ./plugins/aliyun_checker/cmd

FROM opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/csghub_server:base-3.0-runtime

WORKDIR /starhub-bin
ENV GIN_MODE=release
RUN mkdir -p /starhub-bin && \
    curl -fL -o /starhub-bin/GeoLite2-City.mmdb https://opencsg-public-resource.oss-cn-beijing.aliyuncs.com/geoip/GeoLite2-City.mmdb
COPY --from=build-env /go/bin/starhub .
COPY --from=build-env /go/bin/csghub_aliyun_checker /starhub-bin/plugins/csghub_aliyun_checker/v1.0.0/csghub_aliyun_checker
COPY docker/spaces/templates/ /starhub-bin/docker/spaces/templates/
COPY configs/ /starhub-bin/configs/
COPY common/config/config.toml.example /starhub-bin/config/config.toml

EXPOSE 8080
# CMD is args for init.sh
CMD ["/starhub-bin/starhub", "start", "server", "--config=/starhub-bin/config/config.toml"]
