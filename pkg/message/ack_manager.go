package message

import (
	"sync"
	"time"
	"crypto/rand"
	"fmt"

	"github.com/catalystcommunity/muddycore/pkg/logging"
)

const (
	DefaultAckTimeout = 30 * time.Second
	MaxRetries       = 3
	
	// For high-load scenarios, reduce timeout to prevent accumulation
	HighLoadAckTimeout = 10 * time.Second
)

// AckManager handles message acknowledgment tracking
type AckManager struct {
	pendingAcks map[string]*PendingAck
	mutex       sync.RWMutex
	cleanup     chan string
	logger      *logging.Logger
	done        chan bool
	wg          sync.WaitGroup
	closed      bool
}

// NewAckManager creates a new acknowledgment manager
func NewAckManager() *AckManager {
	am := &AckManager{
		pendingAcks: make(map[string]*PendingAck),
		cleanup:     make(chan string, 1000), // Increased buffer for high load scenarios
		done:        make(chan bool),
		logger:      logging.GetDefaultLogger().WithComponent("ack_manager"),
		closed:      false,
	}
	
	// Start cleanup goroutine with proper tracking
	am.wg.Add(1)
	go am.cleanupRoutine()
	
	return am
}

// GenerateMessageID generates a unique message ID
func GenerateMessageID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("msg_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", bytes)
}

// TrackMessage starts tracking a message for acknowledgment
func (am *AckManager) TrackMessage(msg *Message) {
	if !msg.RequiresAck || msg.ID == "" {
		return
	}
	
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	// Check if manager is closed
	if am.closed {
		return
	}
	
	// Set default timeout if not specified
	if msg.AckTimeout == 0 {
		msg.AckTimeout = DefaultAckTimeout
	}
	
	am.pendingAcks[msg.ID] = &PendingAck{
		Message:    msg,
		SentAt:     time.Now(),
		RetryCount: 0,
	}
	
	// Schedule timeout check
	go am.scheduleTimeout(msg.ID, msg.AckTimeout)
}

// ProcessAcknowledgment processes an incoming acknowledgment
func (am *AckManager) ProcessAcknowledgment(ackMsg *AckMessage) bool {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	_, exists := am.pendingAcks[ackMsg.MessageID]
	if !exists {
		// This is expected behavior in high-load scenarios where acks arrive after cleanup
		am.logger.Debug("Received ack for unknown message ID", "message_id", ackMsg.MessageID)
		return false
	}
	
	if !ackMsg.Success {
		am.logger.Error("Received NACK for message", "message_id", ackMsg.MessageID, "error", ackMsg.Error)
	}
	
	// Remove from pending acks
	delete(am.pendingAcks, ackMsg.MessageID)
	
	// Signal cleanup
	select {
	case am.cleanup <- ackMsg.MessageID:
	default:
		// Channel full, cleanup will happen eventually
	}
	
	return ackMsg.Success
}

// GetPendingMessage returns a pending message by ID
func (am *AckManager) GetPendingMessage(messageID string) *Message {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	
	if pending, exists := am.pendingAcks[messageID]; exists {
		return pending.Message
	}
	return nil
}

// GetTimeoutMessages returns messages that have timed out and need retry/failure handling
func (am *AckManager) GetTimeoutMessages() []*PendingAck {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	var timedOut []*PendingAck
	now := time.Now()
	
	for id, pending := range am.pendingAcks {
		if now.Sub(pending.SentAt) >= pending.Message.AckTimeout {
			if pending.RetryCount < MaxRetries {
				// Mark for retry
				pending.RetryCount++
				pending.SentAt = now
				timedOut = append(timedOut, pending)
			} else {
				// Give up on this message
				am.logger.Error("Message failed after max retries", "message_id", id, "max_retries", MaxRetries)
				delete(am.pendingAcks, id)
			}
		}
	}
	
	return timedOut
}

// PendingCount returns the number of messages awaiting acknowledgment
func (am *AckManager) PendingCount() int {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	return len(am.pendingAcks)
}

// Close shuts down the acknowledgment manager
func (am *AckManager) Close() {
	am.mutex.Lock()
	if am.closed {
		am.mutex.Unlock()
		return
	}
	am.closed = true
	am.mutex.Unlock()
	
	close(am.done)
	am.wg.Wait() // Wait for cleanup goroutine to finish
}

// scheduleTimeout schedules a timeout check for a message
func (am *AckManager) scheduleTimeout(messageID string, timeout time.Duration) {
	// Track this goroutine
	am.wg.Add(1)
	defer am.wg.Done()
	
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	
	select {
	case <-timer.C:
		am.mutex.Lock()
		if am.closed {
			am.mutex.Unlock()
			return
		}
		if pending, exists := am.pendingAcks[messageID]; exists {
			if pending.RetryCount < MaxRetries {
				am.logger.Debug("Message timed out, retrying", 
					"message_id", messageID,
					"retry_count", pending.RetryCount+1,
					"max_retries", MaxRetries)
			} else {
				am.logger.Warn("Message timed out, giving up", "message_id", messageID, "max_retries", MaxRetries)
				delete(am.pendingAcks, messageID)
			}
		}
		am.mutex.Unlock()
	case <-am.done:
		return
	}
}

// cleanupRoutine handles cleanup of completed acknowledgments
func (am *AckManager) cleanupRoutine() {
	defer am.wg.Done() // Signal completion when goroutine exits
	
	for {
		select {
		case <-am.cleanup:
			// Cleanup signal received, but actual cleanup already done in ProcessAcknowledgment
		case <-am.done:
			return
		case <-time.After(time.Minute):
			// Periodic cleanup of very old entries (safety net)
			am.periodicCleanup()
		}
	}
}

// periodicCleanup removes very old pending acknowledgments (safety net)
func (am *AckManager) periodicCleanup() {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	cutoff := time.Now().Add(-5 * time.Minute)
	for id, pending := range am.pendingAcks {
		if pending.SentAt.Before(cutoff) && pending.RetryCount >= MaxRetries {
			am.logger.Debug("Cleaning up very old pending ack", "message_id", id)
			delete(am.pendingAcks, id)
		}
	}
}