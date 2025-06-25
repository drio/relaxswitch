package main

import (
	"fmt"
	"os"
	"testing"
)

// MockPlayer implements the Player interface for testing
type MockPlayer struct {
	writes    [][]byte
	closed    bool
	failAfter int // Fail after this many writes to stop playback quickly
}

func (m *MockPlayer) Write(data []byte) (int, error) {
	// Copy the data to avoid slice issues
	copied := make([]byte, len(data))
	copy(copied, data)
	m.writes = append(m.writes, copied)

	// Fail after a few writes to stop playback quickly in tests
	if m.failAfter > 0 && len(m.writes) >= m.failAfter {
		return 0, fmt.Errorf("mock write error - stopping playback for test")
	}

	return len(data), nil
}

func (m *MockPlayer) Close() error {
	m.closed = true
	return nil
}

// Create a mock player that fails quickly for fast tests
func newFastMockPlayer() *MockPlayer {
	return &MockPlayer{failAfter: 3} // Fail after 3 writes
}

func TestLoadConfig(t *testing.T) {
	// Clear all env vars first
	os.Unsetenv("MQTT_PASS")
	os.Unsetenv("MQTT_USER")
	os.Unsetenv("MQTT_URL")
	os.Unsetenv("MQTT_TOPIC")

	config := loadConfig()

	// Check defaults
	if config.MQTTUser != "shelly" {
		t.Errorf("expected MQTTUser to be 'shelly', got '%s'", config.MQTTUser)
	}
	if config.MQTTURL != "tcp://192.168.8.180:1883" {
		t.Errorf("expected MQTTURL to be 'tcp://192.168.8.180:1883', got '%s'", config.MQTTURL)
	}
	if config.MQTTTopic != "shellies/shelly1l-test/relay/0" {
		t.Errorf("expected MQTTTopic to be 'shellies/shelly1l-test/relay/0', got '%s'", config.MQTTTopic)
	}
	if config.MQTTPass != "" {
		t.Errorf("expected MQTTPass to be empty, got '%s'", config.MQTTPass)
	}
}

func TestLoadConfigWithAllEnvVars(t *testing.T) {
	// Set all env vars
	os.Setenv("MQTT_PASS", "testpass")
	os.Setenv("MQTT_USER", "testuser")
	os.Setenv("MQTT_URL", "tcp://testhost:1234")
	os.Setenv("MQTT_TOPIC", "test/topic")

	defer func() {
		os.Unsetenv("MQTT_PASS")
		os.Unsetenv("MQTT_USER")
		os.Unsetenv("MQTT_URL")
		os.Unsetenv("MQTT_TOPIC")
	}()

	config := loadConfig()

	// Check env vars are used
	if config.MQTTPass != "testpass" {
		t.Errorf("expected MQTTPass to be 'testpass', got '%s'", config.MQTTPass)
	}
	if config.MQTTUser != "testuser" {
		t.Errorf("expected MQTTUser to be 'testuser', got '%s'", config.MQTTUser)
	}
	if config.MQTTURL != "tcp://testhost:1234" {
		t.Errorf("expected MQTTURL to be 'tcp://testhost:1234', got '%s'", config.MQTTURL)
	}
	if config.MQTTTopic != "test/topic" {
		t.Errorf("expected MQTTTopic to be 'test/topic', got '%s'", config.MQTTTopic)
	}
}

func TestLoadConfigPartialEnvVars(t *testing.T) {
	// Clear all env vars first
	os.Unsetenv("MQTT_PASS")
	os.Unsetenv("MQTT_USER")
	os.Unsetenv("MQTT_URL")
	os.Unsetenv("MQTT_TOPIC")

	// Set only some env vars
	os.Setenv("MQTT_PASS", "onlypass")
	os.Setenv("MQTT_URL", "tcp://partialhost:5678")

	defer func() {
		os.Unsetenv("MQTT_PASS")
		os.Unsetenv("MQTT_URL")
	}()

	config := loadConfig()

	// Check mix of env vars and defaults
	if config.MQTTPass != "onlypass" {
		t.Errorf("expected MQTTPass to be 'onlypass', got '%s'", config.MQTTPass)
	}
	if config.MQTTURL != "tcp://partialhost:5678" {
		t.Errorf("expected MQTTURL to be 'tcp://partialhost:5678', got '%s'", config.MQTTURL)
	}
	// These should use defaults
	if config.MQTTUser != "shelly" {
		t.Errorf("expected MQTTUser to be 'shelly', got '%s'", config.MQTTUser)
	}
	if config.MQTTTopic != "shellies/shelly1l-test/relay/0" {
		t.Errorf("expected MQTTTopic to be 'shellies/shelly1l-test/relay/0', got '%s'", config.MQTTTopic)
	}
}

func TestLoadConfigEmptyEnvVars(t *testing.T) {
	// Set env vars to empty strings
	os.Setenv("MQTT_PASS", "")
	os.Setenv("MQTT_USER", "")
	os.Setenv("MQTT_URL", "")
	os.Setenv("MQTT_TOPIC", "")

	defer func() {
		os.Unsetenv("MQTT_PASS")
		os.Unsetenv("MQTT_USER")
		os.Unsetenv("MQTT_URL")
		os.Unsetenv("MQTT_TOPIC")
	}()

	config := loadConfig()

	// Empty strings should not override defaults (except for MQTTPass)
	if config.MQTTPass != "" {
		t.Errorf("expected MQTTPass to be empty, got '%s'", config.MQTTPass)
	}
	if config.MQTTUser != "shelly" {
		t.Errorf("expected MQTTUser to be 'shelly', got '%s'", config.MQTTUser)
	}
	if config.MQTTURL != "tcp://192.168.8.180:1883" {
		t.Errorf("expected MQTTURL to be 'tcp://192.168.8.180:1883', got '%s'", config.MQTTURL)
	}
	if config.MQTTTopic != "shellies/shelly1l-test/relay/0" {
		t.Errorf("expected MQTTTopic to be 'shellies/shelly1l-test/relay/0', got '%s'", config.MQTTTopic)
	}
}
