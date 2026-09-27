FROM bufbuild/buf:1.73.0@sha256:75d8f756f0919dcded71e67968a91adfe55fe4dd62c4900c57bbfcd7fe625608 AS buf

FROM golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244 AS build
WORKDIR /src
COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf
COPY contracts/buf.yaml contracts/buf.yaml
COPY contracts/gridos/ contracts/gridos/
COPY contracts/gen/go/go.mod contracts/gen/go/go.sum contracts/gen/go/
COPY infrastructure/aws/go.gen.yaml /tmp/go.gen.yaml
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && go install connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.21.0 && buf generate contracts --template /tmp/go.gen.yaml
COPY services/control/go.mod services/control/go.sum services/control/
RUN cd services/control && GOWORK=off go mod download
COPY services/control/cmd/ services/control/cmd/
COPY services/control/internal/ services/control/internal/
COPY database/migrations/ database/migrations/
COPY database/rollback/ database/rollback/
RUN cd services/control && GOWORK=off go build -trimpath -buildvcs=false -o /out/control ./cmd/control && GOWORK=off go build -trimpath -buildvcs=false -o /out/worker ./cmd/worker && GOWORK=off go build -trimpath -buildvcs=false -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/cc-debian12:nonroot@sha256:9dac0a79194e45a7da0158a9c6da57b217585af0786db3845d1f0ec1a0dd182f AS runtime
WORKDIR /app
COPY --from=build /out/control /app/control
COPY --from=build /out/worker /app/worker
COPY --from=build /out/migrate /app/migrate
COPY --from=build /src/database /app/database
COPY testdata/fleets/ /app/testdata/fleets/
COPY testdata/fixtures/geo/ /app/testdata/fixtures/geo/
COPY testdata/fixtures/public/ /app/testdata/fixtures/public/
USER 65532:65532

FROM runtime AS worker
ENTRYPOINT ["/app/worker"]

FROM runtime AS migrate
ENTRYPOINT ["/app/migrate"]
CMD ["up"]

FROM runtime AS control
EXPOSE 8080
ENTRYPOINT ["/app/control"]
