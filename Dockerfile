FROM golang:1.26.4-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG GIT_SHA
ARG VERSION
ARG BUILD_TIME=unknown

RUN git_sha="${GIT_SHA:-${VERSION:-unknown}}" && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
        -trimpath \
        -ldflags="-s -w -X github.com/tdeshazo/home-api/internal/build.GitSHA=${git_sha} -X github.com/tdeshazo/home-api/internal/build.BuildTime=${BUILD_TIME}" \
        -o /out/api \
        ./cmd/api

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/api /api

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/api"]
