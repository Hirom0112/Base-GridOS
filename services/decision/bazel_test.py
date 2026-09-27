from pathlib import Path

import pytest

if __name__ == "__main__":
    raise SystemExit(
        pytest.main(
            [str(Path(__file__).parent / "gridos"), str(Path(__file__).parent / "tests"), "-q"]
        )
    )
