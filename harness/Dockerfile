FROM python:3.12-slim

WORKDIR /app

# Copy project definition first for better layer caching
COPY pyproject.toml .

# Copy the package source
COPY __init__.py __main__.py api.py config.py runner.py session.py tools.py transcript.py ./harness/

# Install the package and its dependencies
RUN pip install --no-cache-dir .

EXPOSE 8000

CMD ["uvicorn", "harness.api:app", "--host", "0.0.0.0", "--port", "8000"]
