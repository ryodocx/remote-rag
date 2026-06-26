import os
import logging
from opentelemetry import trace, metrics
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.sdk.resources import Resource
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
from opentelemetry.exporter.prometheus import PrometheusMetricReader
from prometheus_client import start_http_server

logger = logging.getLogger(__name__)

def setup_telemetry(service_name="rrag-python", metrics_port=8002):
    resource = Resource(attributes={"service.name": service_name})
    
    # --- Trace Setup (OTLP Push) ---
    endpoint = os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT")
    if endpoint:
        logger.info(f"Setting up OpenTelemetry Traces for service: {service_name}, endpoint: {endpoint}")
        trace_provider = TracerProvider(resource=resource)
        trace.set_tracer_provider(trace_provider)
        otlp_trace_exporter = OTLPSpanExporter(endpoint=f"{endpoint}/v1/traces")
        trace_provider.add_span_processor(BatchSpanProcessor(otlp_trace_exporter))
    else:
        logger.info("OTEL_EXPORTER_OTLP_ENDPOINT not set. Tracing is disabled.")

    # --- Metrics Setup (Prometheus Pull & Optional OTLP Push) ---
    logger.info(f"Starting Prometheus metrics server on port {metrics_port}")
    start_http_server(port=metrics_port, addr="0.0.0.0")
    
    readers = [PrometheusMetricReader()]
    
    if endpoint and os.environ.get("OTEL_EXPORTER_OTLP_METRICS_ENABLED", "").lower() in ("true", "1", "yes"):
        logger.info("OTLP Metrics Push is enabled.")
        otlp_metric_exporter = OTLPMetricExporter(endpoint=f"{endpoint}/v1/metrics")
        readers.append(PeriodicExportingMetricReader(otlp_metric_exporter))

    meter_provider = MeterProvider(resource=resource, metric_readers=readers)
    metrics.set_meter_provider(meter_provider)

def get_tracer(module_name):
    return trace.get_tracer(module_name)

def get_meter(module_name):
    return metrics.get_meter(module_name)
