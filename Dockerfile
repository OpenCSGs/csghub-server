FROM golang:1.21.0

RUN  apt-get update && apt-get install -y telnet jq cron

RUN mkdir -p /hub
WORKDIR /hub
COPY ./ ./
RUN GOOS=linux go build -tags netgo -a -v -o ./starhub /hub/cmd/starhub-server

RUN mkdir -p /starhub-bin/scripts /starhub-bin/builder/store/database/seeds
ENV GIN_MODE=release
RUN cp ./starhub /starhub-bin/ && \
    cp ./scripts/init.sh /starhub-bin/scripts/ &&  \
    cp -r ./builder/store/database/seeds/. /starhub-bin/builder/store/database/seeds/
RUN chmod +x /starhub-bin/scripts/init.sh

RUN rm -rf /hub
WORKDIR /starhub-bin
EXPOSE 8080
ENTRYPOINT ["/starhub-bin/scripts/init.sh"]