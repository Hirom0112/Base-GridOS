import io
import json

import pytest
from gridos.observability import new_provider, start_span


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
