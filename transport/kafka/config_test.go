package kafka

import (
	"testing"
	"time"
)

func TestConfig_validate_MissingBrokers(t *testing.T) {
	t.Parallel()
	cfg := Config{Topic: "test"}
	err := cfg.validate()
	if err != ErrMissingBrokers {
		t.Errorf("expected ErrMissingBrokers, got %v", err)
	}
}

func TestConfig_validate_MissingTopic(t *testing.T) {
	t.Parallel()
	cfg := Config{Brokers: []string{"localhost:9092"}}
	err := cfg.validate()
	if err != ErrMissingTopic {
		t.Errorf("expected ErrMissingTopic, got %v", err)
	}
}

func TestConfig_validate_Success(t *testing.T) {
	t.Parallel()
	cfg := Config{Brokers: []string{"localhost:9092"}, Topic: "test"}
	err := cfg.validate()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestConfig_withDefaults_AllZero(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	cfg.withDefaults()

	if cfg.NumPartitions != 1 {
		t.Errorf("expected NumPartitions=1, got %d", cfg.NumPartitions)
	}
	if cfg.ReplicationFactor != 1 {
		t.Errorf("expected ReplicationFactor=1, got %d", cfg.ReplicationFactor)
	}
	if cfg.MaxBytes != 1e6 {
		t.Errorf("expected MaxBytes=1000000, got %d", cfg.MaxBytes)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout=10s, got %v", cfg.WriteTimeout)
	}
}

func TestConfig_withDefaults_NegativeValues(t *testing.T) {
	t.Parallel()
	cfg := Config{
		NumPartitions:     -1,
		ReplicationFactor: -5,
		MaxBytes:          -100,
		WriteTimeout:      -1 * time.Second,
	}
	cfg.withDefaults()

	if cfg.NumPartitions != 1 {
		t.Errorf("expected NumPartitions=1, got %d", cfg.NumPartitions)
	}
	if cfg.ReplicationFactor != 1 {
		t.Errorf("expected ReplicationFactor=1, got %d", cfg.ReplicationFactor)
	}
	if cfg.MaxBytes != 1e6 {
		t.Errorf("expected MaxBytes=1000000, got %d", cfg.MaxBytes)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout=10s, got %v", cfg.WriteTimeout)
	}
}

func TestConfig_withDefaults_CustomValues(t *testing.T) {
	t.Parallel()
	cfg := Config{
		NumPartitions:     3,
		ReplicationFactor: 2,
		MaxBytes:          2e6,
		WriteTimeout:      5 * time.Second,
	}
	cfg.withDefaults()

	if cfg.NumPartitions != 3 {
		t.Errorf("expected NumPartitions=3, got %d", cfg.NumPartitions)
	}
	if cfg.ReplicationFactor != 2 {
		t.Errorf("expected ReplicationFactor=2, got %d", cfg.ReplicationFactor)
	}
	if cfg.MaxBytes != 2e6 {
		t.Errorf("expected MaxBytes=2000000, got %d", cfg.MaxBytes)
	}
	if cfg.WriteTimeout != 5*time.Second {
		t.Errorf("expected WriteTimeout=5s, got %v", cfg.WriteTimeout)
	}
}
