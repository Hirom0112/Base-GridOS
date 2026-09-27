FROM bufbuild/buf:1.73.0@sha256:75d8f756f0919dcded71e67968a91adfe55fe4dd62c4900c57bbfcd7fe625608 AS buf

FROM python:3.12.13-slim-bookworm@sha256:4766d8b510c428e595d74b9cc5bbb2fae8e26316fffb4adc89908d79aacd58a2 AS build
COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf
COPY --from=ghcr.io/astral-sh/uv:0.11.7@sha256:240fb85ab0f263ef12f492d8476aa3a2e4e1e333f7d67fbdd923d00a506a516a /uv /usr/local/bin/uv
WORKDIR /app
COPY services/decision/pyproject.toml services/decision/uv.lock services/decision/
COPY services/decision/gridos/ services/decision/gridos/
COPY contracts/buf.yaml contracts/buf.yaml
COPY contracts/gridos/ contracts/gridos/
COPY infrastructure/aws/python.gen.yaml /tmp/python.gen.yaml
RUN uv sync --frozen --no-dev --project services/decision
RUN printf '%s\n' '#!/bin/sh' 'exec /app/services/decision/.venv/bin/python -m grpc_tools.protoc "$@"' > /usr/local/bin/protoc && chmod +x /usr/local/bin/protoc && buf generate contracts --template /tmp/python.gen.yaml && /app/services/decision/.venv/bin/python -m grpc_tools.protoc -Icontracts -I/app/services/decision/.venv/lib/python3.12/site-packages/grpc_tools/_proto --grpc_python_out=services/decision/gridos/gen contracts/gridos/v1/*.proto

FROM python:3.12.13-slim-bookworm@sha256:4766d8b510c428e595d74b9cc5bbb2fae8e26316fffb4adc89908d79aacd58a2 AS decision
WORKDIR /app
COPY --from=build /app/services/decision /app/services/decision
COPY testdata/fixtures/public/ /app/testdata/fixtures/public/
ENV PYTHONDONTWRITEBYTECODE=1
USER 65532:65532
EXPOSE 50061
ENTRYPOINT ["/app/services/decision/.venv/bin/python", "-m", "gridos.server", "--port", "50061"]
