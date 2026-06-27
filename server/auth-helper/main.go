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
	"strconv"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
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
	"go.opentelemetry.io/otel/trace"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	cacheInstance       Cache  // キャッシュインターフェースのインスタンス
	oauthIntrospectURL  string // OAuth2.0 Token Introspection エンドポイントのURL
	oauthClientID       string // Introspection用のクライアントID (Basic認証用)
	oauthClientSecret   string // Introspection用のクライアントシークレット (Basic認証用)
	oauthValidationMode string // 検証モード ("introspect" or "jwks")
	jwksInstance        keyfunc.Keyfunc // JWKSインスタンス
	introspectCacheTTL  time.Duration   // Introspectionモード時のキャッシュTTL
	tracer              = otel.Tracer("auth-helper")
	meter               = otel.Meter("auth-helper")
	authCounter         metric.Int64Counter
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

	// 動作モードと共通設定
	oauthValidationMode = os.Getenv("OAUTH_VALIDATION_MODE")
	if oauthValidationMode == "" {
		oauthValidationMode = "introspect"
	}

	ttlStr := os.Getenv("AUTH_INTROSPECT_CACHE_TTL_SECONDS")
	if ttlStr != "" {
		if ttl, err := strconv.Atoi(ttlStr); err == nil {
			introspectCacheTTL = time.Duration(ttl) * time.Second
		} else {
			introspectCacheTTL = 60 * time.Second
		}
	} else {
		introspectCacheTTL = 60 * time.Second
	}

	if oauthValidationMode == "jwks" {
		jwksURL := os.Getenv("OAUTH_JWKS_URL")
		if jwksURL == "" {
			log.Fatal("OAUTH_JWKS_URL is required when OAUTH_VALIDATION_MODE is jwks")
		}
		
		options := keyfunc.Options{
			RefreshInterval: time.Hour,
			RefreshRateLimit: 5 * time.Minute,
		}
		var err error
		jwksInstance, err = keyfunc.NewDefault([]string{jwksURL}, options)
		if err != nil {
			log.Fatalf("Failed to create JWKS from URL: %v", err)
		}
		log.Printf("JWKS mode enabled. JWKS URL: %s", jwksURL)
	} else {
		// Introspection用の設定を環境変数から取得します
		oauthIntrospectURL = os.Getenv("OAUTH_INTROSPECT_URL")
		oauthClientID = os.Getenv("OAUTH_CLIENT_ID")
		oauthClientSecret = os.Getenv("OAUTH_CLIENT_SECRET")

		if oauthIntrospectURL == "" || oauthClientID == "" || oauthClientSecret == "" {
			log.Println("WARNING: OAUTH_INTROSPECT_URL, OAUTH_CLIENT_ID, or OAUTH_CLIENT_SECRET is missing.")
		}
		log.Printf("Introspection mode enabled. Cache TTL: %v", introspectCacheTTL)
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

type IntrospectionResponse struct {
	Active   bool        `json:"active"`
	Iss      string      `json:"iss"`
	Aud      interface{} `json:"aud"`
	ClientID string      `json:"client_id"`
	Scope    string      `json:"scope"`
	Sub      string      `json:"sub"`
	Email    string      `json:"email"`
	Groups   interface{} `json:"groups"`
}

func validateClientConstraints(result *IntrospectionResponse, span trace.Span) bool {
	expectedIss := os.Getenv("AUTH_FILTER_ISS")
	if expectedIss != "" && result.Iss != expectedIss {
		span.SetAttributes(attribute.String("auth.reason", "filtered_by_iss"))
		return false
	}

	expectedAud := os.Getenv("AUTH_FILTER_AUD")
	if expectedAud != "" {
		audMatched := false
		switch v := result.Aud.(type) {
		case string:
			if v == expectedAud {
				audMatched = true
			}
		case []interface{}:
			for _, a := range v {
				if s, ok := a.(string); ok && s == expectedAud {
					audMatched = true
					break
				}
			}
		}
		if !audMatched {
			span.SetAttributes(attribute.String("auth.reason", "filtered_by_aud"))
			return false
		}
	}

	expectedClient := os.Getenv("AUTH_FILTER_CLIENT_ID")
	if expectedClient != "" && result.ClientID != expectedClient {
		span.SetAttributes(attribute.String("auth.reason", "filtered_by_client_id"))
		return false
	}

	expectedScope := os.Getenv("AUTH_FILTER_SCOPES")
	if expectedScope != "" {
		scopes := strings.Split(result.Scope, " ")
		scopeMatched := false
		for _, s := range scopes {
			if s == expectedScope {
				scopeMatched = true
				break
			}
		}
		if !scopeMatched {
			span.SetAttributes(attribute.String("auth.reason", "filtered_by_scope"))
			return false
		}
	}

	return true
}

func validateUserConstraints(result *IntrospectionResponse, span trace.Span) bool {
	domainsStr := os.Getenv("AUTH_FILTER_EMAIL_DOMAINS")
	if domainsStr != "" {
		domains := strings.Split(domainsStr, ",")
		domainMatched := false
		for _, domain := range domains {
			if strings.HasSuffix(result.Email, strings.TrimSpace(domain)) {
				domainMatched = true
				break
			}
		}
		if !domainMatched {
			span.SetAttributes(attribute.String("auth.reason", "filtered_by_email_domain"))
			return false
		}
	}

	emailsStr := os.Getenv("AUTH_FILTER_EMAILS")
	if emailsStr != "" {
		emails := strings.Split(emailsStr, ",")
		emailMatched := false
		for _, email := range emails {
			if result.Email == strings.TrimSpace(email) {
				emailMatched = true
				break
			}
		}
		if !emailMatched {
			span.SetAttributes(attribute.String("auth.reason", "filtered_by_email"))
			return false
		}
	}

	groupsStr := os.Getenv("AUTH_FILTER_GROUPS")
	if groupsStr != "" {
		allowedGroups := strings.Split(groupsStr, ",")
		groupMatched := false
		switch v := result.Groups.(type) {
		case string:
			for _, ag := range allowedGroups {
				if v == strings.TrimSpace(ag) {
					groupMatched = true
					break
				}
			}
		case []interface{}:
			for _, g := range v {
				if sg, ok := g.(string); ok {
					for _, ag := range allowedGroups {
						if sg == strings.TrimSpace(ag) {
							groupMatched = true
							break
						}
					}
				}
			}
		}
		if !groupMatched {
			span.SetAttributes(attribute.String("auth.reason", "filtered_by_groups"))
			return false
		}
	}

	subsStr := os.Getenv("AUTH_FILTER_SUBJECTS")
	if subsStr != "" {
		subs := strings.Split(subsStr, ",")
		subMatched := false
		for _, sub := range subs {
			if result.Sub == strings.TrimSpace(sub) {
				subMatched = true
				break
			}
		}
		if !subMatched {
			span.SetAttributes(attribute.String("auth.reason", "filtered_by_subject"))
			return false
		}
	}

	return true
}

// introspectToken はRFC7662に基づいてトークンの有効性を確認します。
func introspectToken(ctx context.Context, token string) (*IntrospectionResponse, error) {
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
		return nil, err
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Introspection failed with status %d: %s", resp.StatusCode, string(body))
		return nil, nil
	}

	var result IntrospectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.Bool("introspection.active", result.Active))
	return &result, nil
}

func verifyJWT(ctx context.Context, tokenString string) (*IntrospectionResponse, error) {
	ctx, span := tracer.Start(ctx, "verifyJWT")
	defer span.End()

	token, err := jwt.Parse(tokenString, jwksInstance.Keyfunc)
	if err != nil {
		span.SetAttributes(attribute.String("error.reason", "jwt_parse_error"))
		return nil, err
	}

	if !token.Valid {
		span.SetAttributes(attribute.String("error.reason", "invalid_jwt"))
		return nil, errors.New("invalid jwt")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		span.SetAttributes(attribute.String("error.reason", "invalid_claims_type"))
		return nil, errors.New("invalid claims type")
	}

	resp := &IntrospectionResponse{Active: true}
	
	if iss, ok := claims["iss"].(string); ok {
		resp.Iss = iss
	}
	if aud, ok := claims["aud"]; ok {
		resp.Aud = aud
	}
	if clientID, ok := claims["client_id"].(string); ok {
		resp.ClientID = clientID
	} else if cid, ok := claims["cid"].(string); ok {
		resp.ClientID = cid
	}
	if scope, ok := claims["scope"].(string); ok {
		resp.Scope = scope
	}
	if sub, ok := claims["sub"].(string); ok {
		resp.Sub = sub
	}
	if email, ok := claims["email"].(string); ok {
		resp.Email = email
	}
	if groups, ok := claims["groups"]; ok {
		resp.Groups = groups
	}

	return resp, nil
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

	var resp *IntrospectionResponse

	if oauthValidationMode == "jwks" {
		resp, err = verifyJWT(ctx, token)
		if err != nil {
			log.Printf("Error verifying JWT: %v", err)
			span.RecordError(err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	} else {
		val, err := cacheInstance.Get(ctx, tokenHash)
		if err == nil && val == "valid" {
			span.SetAttributes(attribute.Bool("cache.hit", true))
			w.WriteHeader(http.StatusOK)
			return
		}
		span.SetAttributes(attribute.Bool("cache.hit", false))

		resp, err = introspectToken(ctx, token)
		if err != nil {
			log.Printf("Error introspecting token: %v", err)
			span.RecordError(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if resp != nil && resp.Active {
		if validateClientConstraints(resp, span) && validateUserConstraints(resp, span) {
			if oauthValidationMode != "jwks" {
				cacheInstance.Set(ctx, tokenHash, "valid", introspectCacheTTL)
			}
			span.SetAttributes(attribute.String("auth.status", "authorized"))
			if authCounter != nil {
				authCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "authorized")))
			}
			w.WriteHeader(http.StatusOK)
			return
		}
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
