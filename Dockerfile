# syntax=docker/dockerfile:1

FROM golang:1.27.1-alpine AS build

ARG VERSION=dev
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /sitechecker ./cmd/sitechecker

FROM alpine:3.23

RUN apk add --no-cache ca-certificates
COPY --from=build /sitechecker /usr/local/bin/sitechecker

USER 65534:65534
ENTRYPOINT ["/usr/local/bin/sitechecker"]
