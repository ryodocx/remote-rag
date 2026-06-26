FROM python:3.11-slim

# Set working directory
WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements and install python packages
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy the source code
COPY src/ src/
COPY search_cli.py .

# Ensure data directory exists
RUN mkdir -p /app/data

# By default, drop into a bash shell so users can run CLI, ingestion, or the MCP server
CMD ["bash"]
