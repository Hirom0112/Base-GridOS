import json
import re
from collections.abc import Iterator, Mapping, Sequence
from contextlib import contextmanager
from typing import TextIO

from opentelemetry.sdk.trace import ReadableSpan, TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor, SpanExporter, SpanExportResult
from opentelemetry.trace import Span


def _safe_identity(value: str) -> bool:
    if re.fullmatch(r"[A-Za-z0-9_-]{1,128}", value) is None:
        return False
    lowered = value.lower()
    return not any(
        part in lowered
        for part in ("site-", "device-", "member-", "credential", "travel", "bearer")
    )


class _SafeExporter(SpanExporter):
    def __init__(self, writer: TextIO) -> None:
        self._writer = writer

    def export(self, spans: Sequence[ReadableSpan]) -> SpanExportResult:
        for span in spans:
            attributes = span.attributes or {}
            correlation_id = attributes.get("correlation_id", "unavailable")
            workflow_id = attributes.get("workflow_id", "unavailable")
            record = {
                "name": "gridos.decision",
                "trace_id": f"{span.context.trace_id:032x}"
                if span.context is not None
                else "unavailable",
                "correlation_id": correlation_id
                if isinstance(correlation_id, str) and _safe_identity(correlation_id)
                else "redacted",
                "workflow_id": workflow_id
                if isinstance(workflow_id, str) and _safe_identity(workflow_id)
                else "redacted",
            }
            self._writer.write(json.dumps(record, separators=(",", ":")) + "\n")
        return SpanExportResult.SUCCESS

    def shutdown(self) -> None:
        self._writer.flush()


def new_provider(writer: TextIO) -> TracerProvider:
    provider = TracerProvider()
    provider.add_span_processor(SimpleSpanProcessor(_SafeExporter(writer)))
    return provider


@contextmanager
def start_span(
    provider: TracerProvider,
    name: str,
    correlation_id: str,
    workflow_id: str,
    fields: Mapping[str, str],
) -> Iterator[Span]:
    if not _safe_identity(correlation_id) or not _safe_identity(workflow_id):
        raise ValueError("invalid trace identity")
    with provider.get_tracer("gridos.decision").start_as_current_span(name) as span:
        span.set_attribute("correlation_id", correlation_id)
        span.set_attribute("workflow_id", workflow_id)
        for key, value in fields.items():
            span.set_attribute(key, value)
        yield span
