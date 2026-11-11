# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH VERSION=dev
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /out/koito-card .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/koito-card /koito-card
EXPOSE 8080
HEALTHCHECK --interval=60s --timeout=5s --start-period=5s CMD ["/koito-card", "healthcheck"]
ENTRYPOINT ["/koito-card"]
