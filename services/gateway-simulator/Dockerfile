FROM bufbuild/buf:1.73.0@sha256:75d8f756f0919dcded71e67968a91adfe55fe4dd62c4900c57bbfcd7fe625608 AS buf

FROM golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244 AS build
WORKDIR /src
COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf
COPY contracts/buf.yaml contracts/buf.yaml
COPY contracts/gridos/ contracts/gridos/
COPY contracts/gen/go/go.mod contracts/gen/go/go.sum contracts/gen/go/
COPY infrastructure/aws/go.gen.yaml /tmp/go.gen.yaml
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && go install connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.21.0 && buf generate contracts --template /tmp/go.gen.yaml
COPY services/gateway-simulator/go.mod services/gateway-simulator/go.sum services/gateway-simulator/
RUN cd services/gateway-simulator && GOWORK=off go mod download
COPY services/gateway-simulator/cmd/ services/gateway-simulator/cmd/
COPY services/gateway-simulator/internal/ services/gateway-simulator/internal/
RUN cd services/gateway-simulator && GOWORK=off CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/gateway-simulator ./cmd/gateway-simulator

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS gateway
WORKDIR /app
COPY --from=build /out/gateway-simulator /app/gateway-simulator
COPY testdata/fleets/ /app/testdata/fleets/
COPY testdata/scenarios/ /app/testdata/scenarios/
USER 65532:65532
EXPOSE 8081
ENTRYPOINT ["/app/gateway-simulator"]
