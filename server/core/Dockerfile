FROM python:3.14-slim

# Set working directory
WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements and install python packages
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Create a non-root user and set up data directory
RUN useradd -m appuser && \
    mkdir -p /app/data && \
    chown -R appuser:appuser /app

USER appuser

# Copy the source code with correct ownership
COPY --chown=appuser:appuser src/ src/
COPY --chown=appuser:appuser search_cli.py .

# By default, drop into a bash shell so users can run CLI, ingestion, or the MCP server
CMD ["bash"]
