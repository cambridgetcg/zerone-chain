FROM python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev gcc && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY pyproject.toml ./
COPY transforms/ ./transforms/
COPY trainer/ ./trainer/
COPY configs/ ./configs/

RUN pip install --no-cache-dir -e .

ENTRYPOINT ["transform"]
CMD ["--help"]
