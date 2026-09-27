import io
import json
from typing import cast
from urllib.request import urlopen

import grpc
import pytest

from gridos.observability import new_provider, start_metrics_server, start_span, traced_rpc


def test_python_trace_scrubs_private_fields() -> None:
    output = io.StringIO()
    provider = new_provider(output)
    with start_span(
        provider,
        "dispatch.optimize",
        "correlation-1",
        "workflow-1",
        {
            "site_id": "site-private-123",
            "command_credential": "credential-private-456",
            "travel_window": "travel-window-private-789",
        },
    ):
        pass
    provider.shutdown()
    exported = output.getvalue()
    for private in ("site-private-123", "credential-private-456", "travel-window-private-789"):
        assert private not in exported
    record = json.loads(exported)
    assert record["correlation_id"] == "correlation-1"
    assert record["workflow_id"] == "workflow-1"


def test_python_trace_rejects_private_identity() -> None:
    output = io.StringIO()
    provider = new_provider(output)
    with pytest.raises(ValueError, match="trace identity"):
        with start_span(provider, "dispatch.optimize", "site-private-123", "workflow-1", {}):
            pass
    provider.shutdown()


def test_rpc_trace_uses_request_metadata_and_calls_handler() -> None:
    output = io.StringIO()
    provider = new_provider(output)

    class Context:
        def invocation_metadata(self) -> tuple[tuple[str, str], ...]:
            return (("x-correlation-id", "correlation-7"), ("x-workflow-id", "workflow-7"))

    def handler(request: str, context: grpc.ServicerContext) -> str:
        assert request == "request"
        assert dict(context.invocation_metadata())["x-workflow-id"] == "workflow-7"
        return "response"

    wrapped = traced_rpc(provider, "Optimize", handler)
    assert wrapped("request", cast(grpc.ServicerContext, Context())) == "response"
    provider.shutdown()
    record = json.loads(output.getvalue())
    assert record["correlation_id"] == "correlation-7"
    assert record["workflow_id"] == "workflow-7"
    assert "private-site" not in output.getvalue()


def test_metrics_endpoint_counts_served_rpc() -> None:
    output = io.StringIO()
    provider = new_provider(output)

    class Context:
        def invocation_metadata(self) -> tuple[tuple[str, str], ...]:
            return ()

    wrapped = traced_rpc(provider, "Forecast", lambda request, context: "forecast")
    assert wrapped("request", cast(grpc.ServicerContext, Context())) == "forecast"
    server, _ = start_metrics_server(0)
    try:
        with urlopen(f"http://127.0.0.1:{server.server_port}/metrics", timeout=2) as response:
            metrics = response.read().decode()
        assert 'gridos_decision_rpc_total{method="Forecast"} 1.0' in metrics
    finally:
        server.shutdown()
        server.server_close()
        provider.shutdown()
