FROM golang:1.18-alpine AS builder
WORKDIR /src
ADD . .
ARG VERSION=dev
RUN apk add git
RUN go build -ldflags "-X main.GitCommit=$VERSION" -o /app

FROM alpine:3.15.4
COPY --from=builder /app /usr/bin/app
RUN apk add --no-cache tzdata
ENTRYPOINT ["app"]
