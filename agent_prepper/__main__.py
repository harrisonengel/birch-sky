"""Local dev entry point: `python -m agent_prepper` runs the FastAPI app."""

from __future__ import annotations

import os

import uvicorn


def main() -> None:
    port = int(os.environ.get("PORT", "8002"))
    uvicorn.run(
        "agent_prepper.api:app",
        host="0.0.0.0",
        port=port,
        reload=False,
    )


if __name__ == "__main__":
    main()
