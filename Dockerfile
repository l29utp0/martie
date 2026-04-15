FROM golang:1.26.2-trixie AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

ARG TARGETOS=linux
ARG TARGETARCH

RUN arch="${TARGETARCH:-$(go env GOARCH)}" \
	&& CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$arch" \
	go build -mod=readonly -trimpath -buildvcs=false -ldflags="-s -w" -o /out/martie ./cmd/martie \
	&& mkdir -p /out/data

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/martie /usr/local/bin/martie
COPY --from=build --chown=65532:65532 /out/data /data

USER 65532:65532

ENV SQLITE_PATH=/data/bot.db

STOPSIGNAL SIGTERM

ENTRYPOINT ["/usr/local/bin/martie"]
CMD ["run"]
