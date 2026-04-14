FROM python:3.12-slim

WORKDIR /app

# Copy the package source into a properly-named subdirectory so that
# `agent_prepper` is importable. pyproject.toml lives at the project root
# inside the image.
COPY pyproject.toml ./
COPY __init__.py __main__.py api.py config.py prompts.py runner.py session.py ./agent_prepper/

RUN pip install --no-cache-dir .

EXPOSE 8002

CMD ["uvicorn", "agent_prepper.api:app", "--host", "0.0.0.0", "--port", "8002"]
