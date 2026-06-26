package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	cacheInstance      Cache  // キャッシュインターフェースのインスタンス
	oauthIntrospectURL string // OAuth2.0 Token Introspection エンドポイントのURL
	oauthClientID      string // Introspection用のクライアントID (Basic認証用)
	oauthClientSecret  string // Introspection用のクライアントシークレット (Basic認証用)
	tracer             = otel.Tracer("auth-helper")
	meter              = otel.Meter("auth-helper")
	authCounter        metric.Int64Counter
)

func init() {
	// 環境変数に基づいて使用するキャッシュ機構を決定します
	cacheType := os.Getenv("CACHE_TYPE")
	if cacheType == "memory" {
		log.Println("Using In-Memory Cache")
		cacheInstance = NewMemoryCache()
	} else {
		log.Println("Using Redis Cache")
		cacheInstance = NewRedisCache(os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT"))
	}

	// Introspection用の設定を環境変数から取得します
	oauthIntrospectURL = os.Getenv("OAUTH_INTROSPECT_URL")
	oauthClientID = os.Getenv("OAUTH_CLIENT_ID")
	oauthClientSecret = os.Getenv("OAUTH_CLIENT_SECRET")

	if oauthIntrospectURL == "" || oauthClientID == "" || oauthClientSecret == "" {
		log.Println("WARNING: OAUTH_INTROSPECT_URL, OAUTH_CLIENT_ID, or OAUTH_CLIENT_SECRET is missing.")
	}
}

func initTelemetry() (*sdktrace.TracerProvider, *sdkmetric.MeterProvider) {
	res, err := resource.New(context.Background(),
		resource.WithAttributes(attribute.String("service.name", "rrag-auth-helper")),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	// --- Trace Setup ---
	var tp *sdktrace.TracerProvider
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint != "" {
		traceExporter, err := otlptracehttp.New(context.Background())
		if err != nil {
			log.Fatalf("failed to create OTLP trace exporter: %v", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.TraceContext{})
	} else {
		log.Println("OTEL_EXPORTER_OTLP_ENDPOINT not set. Tracing is disabled.")
	}

	// --- Metric Setup ---
	metricExporter, err := prometheus.New()
	if err != nil {
		log.Fatalf("failed to create Prometheus metric exporter: %v", err)
	}

	meterOptions := []sdkmetric.Option{
		sdkmetric.WithReader(metricExporter),
		sdkmetric.WithResource(res),
	}

	if endpoint != "" {
		enabled := strings.ToLower(os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENABLED"))
		if enabled == "true" || enabled == "1" || enabled == "yes" {
			otlpMetricExporter, err := otlpmetrichttp.New(context.Background())
			if err != nil {
				log.Fatalf("failed to create OTLP metric exporter: %v", err)
			}
			meterOptions = append(meterOptions, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(otlpMetricExporter)))
			log.Println("OTLP Metrics Push is enabled.")
		}
	}

	// Setup Meter Provider
	mp := sdkmetric.NewMeterProvider(meterOptions...)
	otel.SetMeterProvider(mp)
	
	// Initialize custom metrics
	meter = otel.Meter("auth-helper")
	authCounter, _ = meter.Int64Counter("auth.requests", metric.WithDescription("Number of auth requests"))

	return tp, mp
}

func hashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}

func introspectToken(ctx context.Context, token string) (bool, error) {
	ctx, span := tracer.Start(ctx, "introspectToken")
	defer span.End()

	if oauthIntrospectURL == "" {
		if os.Getenv("MOCK_AUTH") == "true" {
			log.Println("Mocking introspection: Returning true (MOCK_AUTH is true)")
			return true, nil
		}
		log.Println("OAUTH_INTROSPECT_URL is missing and MOCK_AUTH is not true. Rejecting token.")
		return false, nil
	}

	data := url.Values{}
	data.Set("token", token)
	data.Set("token_type_hint", "access_token")

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	var resp *http.Response
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		req, _ := http.NewRequestWithContext(ctx, "POST", oauthIntrospectURL, strings.NewReader(data.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		
		auth := oauthClientID + ":" + oauthClientSecret
		basicAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Set("Authorization", "Basic "+basicAuth)

		resp, err = client.Do(req)
		
		if err == nil && resp.StatusCode < 500 {
			break
		}
		
		log.Printf("Introspection attempt %d failed, retrying...", i+1)
		if resp != nil {
			resp.Body.Close()
		}
		
		if i < maxRetries-1 {
			time.Sleep(time.Duration(1<<i) * 200 * time.Millisecond)
		}
	}

	if err != nil {
		span.SetAttributes(attribute.String("error.reason", "max_retries_exceeded"))
		return false, err
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Introspection failed with status %d: %s", resp.StatusCode, string(body))
		return false, nil
	}

	var result struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	span.SetAttributes(attribute.Bool("introspection.active", result.Active))
	return result.Active, nil
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "authHandler_logic")
	defer span.End()

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		span.SetAttributes(attribute.String("auth.reason", "missing_bearer"))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		span.SetAttributes(attribute.String("auth.reason", "empty_token"))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	tokenHash := hashToken(token)

	val, err := cacheInstance.Get(ctx, tokenHash)
	if err == nil && val == "valid" {
		span.SetAttributes(attribute.Bool("cache.hit", true))
		w.WriteHeader(http.StatusOK)
		return
	}
	span.SetAttributes(attribute.Bool("cache.hit", false))

	active, err := introspectToken(ctx, token)
	if err != nil {
		log.Printf("Error introspecting token: %v", err)
		span.RecordError(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if active {
		cacheInstance.Set(ctx, tokenHash, "valid", 60*time.Second)
		span.SetAttributes(attribute.String("auth.status", "authorized"))
		if authCounter != nil {
			authCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "authorized")))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	span.SetAttributes(attribute.String("auth.status", "unauthorized"))
	if authCounter != nil {
		authCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "unauthorized")))
	}
	w.WriteHeader(http.StatusUnauthorized)
}

func main() {
	tp, mp := initTelemetry()
	if tp != nil {
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				log.Printf("Error shutting down tracer provider: %v", err)
			}
			if mp != nil {
				if err := mp.Shutdown(context.Background()); err != nil {
					log.Printf("Error shutting down meter provider: %v", err)
				}
			}
		}()
	}

	handler := http.HandlerFunc(authHandler)
	wrappedHandler := otelhttp.NewHandler(handler, "auth_endpoint")

	http.Handle("/auth", wrappedHandler)
	http.Handle("/metrics", promhttp.Handler())
	
	port := "8000"
	log.Printf("Auth Helper starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
