#!/usr/bin/env python3
"""XHub E2E 唯一入口；业务场景与生命周期位于 e2e/acceptance。"""
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "e2e"))
from acceptance.cli import main

if __name__ == "__main__":
    raise SystemExit(main())

