FROM debian:10

ADD ./nebula.linux /app/nebula
ADD ./config.toml /app/config.toml
ADD ./front /app/front
ADD ./templates /app/templates

WORKDIR /app

ENTRYPOINT ["/app/nebula"]

