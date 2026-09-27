import sys
from pathlib import Path

import grpc_tools
from grpc_tools import protoc


def main() -> int:
    output = Path(sys.argv[1])
    output.mkdir(parents=True, exist_ok=True)
    include = Path(grpc_tools.__file__).parent / "_proto"
    return protoc.main(
        [
            "protoc",
            "-Icontracts",
            f"-I{include}",
            f"--python_out={output}",
            f"--grpc_python_out={output}",
            *sys.argv[2:],
        ]
    )


if __name__ == "__main__":
    raise SystemExit(main())
