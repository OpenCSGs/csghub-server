FROM opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/csghub_server:base-1.1 AS build-env
WORKDIR /starhub
COPY . .
RUN  CGO_ENABLED=1 GOOS=linux go build -ldflags "-X opencsg.com/csghub-server/version.GitRevision=$(git rev-parse --short HEAD) -X opencsg.com/csghub-server/version.StarhubAPIVersion=$CI_COMMIT_TAG"  -v -o /go/bin/starhub ./cmd/csghub-server && \
     rm -rf /go/pkg && \
     rm -rf /starhub

FROM opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/csghub_server:base-1.1
WORKDIR /starhub-bin
ENV GIN_MODE=release
COPY --from=build-env /go/bin/starhub .
COPY scripts/init.sh /starhub-bin/scripts/
COPY docker/spaces/templates/ /starhub-bin/docker/spaces/templates/
COPY configs/ /starhub-bin/configs/
RUN chmod +x /starhub-bin/scripts/init.sh

EXPOSE 8080
ENTRYPOINT ["/starhub-bin/scripts/init.sh"]
