FROM golang:1.25.7-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/catalog ./cmd/catalog

FROM scratch
WORKDIR /app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/ /app/
COPY data/ /app/data/
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=4s --start-period=10s CMD ["/app/server", "healthcheck"]
CMD ["/app/server"]
