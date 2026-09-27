FROM python:3.12.13-slim-bookworm@sha256:4766d8b510c428e595d74b9cc5bbb2fae8e26316fffb4adc89908d79aacd58a2 AS build
COPY --from=ghcr.io/astral-sh/uv:0.11.7@sha256:240fb85ab0f263ef12f492d8476aa3a2e4e1e333f7d67fbdd923d00a506a516a /uv /usr/local/bin/uv
WORKDIR /app
COPY services/decision/pyproject.toml services/decision/uv.lock services/decision/
COPY services/decision/gridos/ services/decision/gridos/
RUN uv sync --frozen --no-dev --project services/decision

FROM python:3.12.13-slim-bookworm@sha256:4766d8b510c428e595d74b9cc5bbb2fae8e26316fffb4adc89908d79aacd58a2 AS decision
WORKDIR /app
COPY --from=build /app/services/decision /app/services/decision
COPY testdata/fixtures/public/ /app/testdata/fixtures/public/
ENV PYTHONDONTWRITEBYTECODE=1
USER 65532:65532
EXPOSE 50061
ENTRYPOINT ["/app/services/decision/.venv/bin/python", "-m", "gridos.server", "--port", "50061"]
