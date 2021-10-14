FROM golang:1.17.1-alpine AS builder
WORKDIR /src
ADD . .
ARG VERSION=dev
RUN go build -ldflags "-X main.GitCommit=$VERSION" -o /app

FROM alpine:3.14.2
COPY --from=builder /app /usr/bin/app
RUN apk add --no-cache tzdata
ENTRYPOINT ["app"]
