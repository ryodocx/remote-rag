package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryCache_GetSet(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Close()

	ctx := context.Background()

	// Miss check
	_, err := cache.Get(ctx, "non-existent")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Expected ErrCacheMiss, got %v", err)
	}

	// Set and Get check
	err = cache.Set(ctx, "key1", "value1", 10*time.Second)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("Expected value1, got %s", val)
	}
}

func TestMemoryCache_Expiration(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Close()

	ctx := context.Background()

	// Set with very short TTL
	err := cache.Set(ctx, "key-expire", "value-expire", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Immediate Get should succeed
	val, err := cache.Get(ctx, "key-expire")
	if err != nil {
		t.Fatalf("Immediate Get failed: %v", err)
	}
	if val != "value-expire" {
		t.Errorf("Expected value-expire, got %s", val)
	}

	// Wait for TTL to expire
	time.Sleep(30 * time.Millisecond)

	// Should miss
	_, err = cache.Get(ctx, "key-expire")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Expected ErrCacheMiss after expiration, got %v", err)
	}
}

func TestMemoryCache_Close(t *testing.T) {
	cache := NewMemoryCache()
	
	// Set a key
	ctx := context.Background()
	cache.Set(ctx, "key", "val", 1*time.Minute)

	err := cache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// The map should still be queryable (since Close doesn't destroy the map, it just stops the cleanup goroutine)
	val, err := cache.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get failed after Close: %v", err)
	}
	if val != "val" {
		t.Errorf("Expected val, got %s", val)
	}
}
