import json
from pathlib import Path

from google.protobuf.json_format import MessageToDict, Parse
from gridos.v1.dispatch_pb2 import CommandIntent


def test_command_intent_canonical_json() -> None:
    fixture_path = (
        Path(__file__).parents[4] / "testdata" / "fixtures" / "contracts" / "command_intent.json"
    )
    fixture = fixture_path.read_text()
    message = Parse(fixture, CommandIntent())
    canonical = json.dumps(MessageToDict(message), separators=(", ", ":")) + "\n"
    assert canonical == fixture
