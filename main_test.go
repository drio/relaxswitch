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

func TestDoneChanSynchronization(t *testing.T) {
	playerFactory := func() (Player, error) {
		return newFastMockPlayer(), nil
	}
	am := NewAudioManager(playerFactory)

	// Start playback
	if err := am.playEmbeddedMP3(0); err != nil {
		t.Fatalf("failed to start playback: %v", err)
	}

	// Verify goroutine is running
	if !am.playing {
		t.Error("expected playing to be true")
	}
	if am.doneChan == nil {
		t.Error("expected doneChan to be set")
	}

	// Capture the doneChan reference before stopping
	doneChan := am.doneChan

	// Test that doneChan is NOT closed yet (goroutine still running)
	select {
	case <-doneChan:
		t.Error("doneChan should not be closed while goroutine is running")
	default:
		// Expected - channel not closed yet
	}

	// Stop audio - this should wait for goroutine to finish
	am.stopAudio()

	// Verify doneChan was closed (indicating goroutine finished)
	select {
	case <-doneChan:
		// Expected - channel should be closed now
	default:
		t.Error("doneChan should be closed after stopAudio completes")
	}

	// Verify cleanup happened
	if am.playing {
		t.Error("expected playing to be false after stop")
	}
	if am.player != nil {
		t.Error("expected player to be nil after stop")
	}
	if am.doneChan != nil {
		t.Error("expected doneChan to be nil after stop")
	}
}

// MockMessage implements the mqtt.Message interface for testing
type MockMessage struct {
	topic   string
	payload []byte
}

func (m *MockMessage) Duplicate() bool   { return false }
func (m *MockMessage) Qos() byte         { return 0 }
func (m *MockMessage) Retained() bool    { return false }
func (m *MockMessage) Topic() string     { return m.topic }
func (m *MockMessage) MessageID() uint16 { return 0 }
func (m *MockMessage) Payload() []byte   { return m.payload }
func (m *MockMessage) Ack()              {}

func TestDuplicateStateIgnored(t *testing.T) {
	playerFactory := func() (Player, error) {
		return newFastMockPlayer(), nil
	}
	am := NewAudioManager(playerFactory)

	handler := createMessageHandler(am)

	// Send first "on" message
	msg1 := &MockMessage{topic: "test/topic", payload: []byte("on")}
	handler(nil, msg1)

	// Verify it was processed
	if am.lastState != "on" {
		t.Errorf("expected lastState to be 'on', got '%s'", am.lastState)
	}
	if !am.playing {
		t.Error("expected playing to be true after first 'on' message")
	}

	// Send duplicate "on" message (should be ignored)
	msg2 := &MockMessage{topic: "test/topic", payload: []byte("on")}
	handler(nil, msg2)

	// State should remain the same
	if am.lastState != "on" {
		t.Errorf("expected lastState to remain 'on', got '%s'", am.lastState)
	}
	if !am.playing {
		t.Error("expected playing to remain true after duplicate 'on' message")
	}

	// Send "off" message
	msg3 := &MockMessage{topic: "test/topic", payload: []byte("off")}
	handler(nil, msg3)

	// Verify state changed
	if am.lastState != "off" {
		t.Errorf("expected lastState to be 'off', got '%s'", am.lastState)
	}
	if am.playing {
		t.Error("expected playing to be false after 'off' message")
	}

	// Send duplicate "off" message (should be ignored)
	msg4 := &MockMessage{topic: "test/topic", payload: []byte("off")}
	handler(nil, msg4)

	// State should remain the same
	if am.lastState != "off" {
		t.Errorf("expected lastState to remain 'off', got '%s'", am.lastState)
	}
	if am.playing {
		t.Error("expected playing to remain false after duplicate 'off' message")
	}
}
