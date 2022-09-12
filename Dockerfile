# syntax=docker/dockerfile:1

## Build
FROM golang:1.16-buster AS build

WORKDIR /
COPY go.mod /
COPY go.sum /
RUN go mod download

COPY *.go /

RUN go build -o /snapcraft-exporter

## Deploy
FROM ubuntu:jammy

WORKDIR /

RUN apt-get update -y && apt-get install -y snapcraft

COPY --from=build /snapcraft-exporter /snapcraft-exporter

EXPOSE 9888

ENTRYPOINT ["/snapcraft-exporter"]
