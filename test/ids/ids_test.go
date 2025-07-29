package ids_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/logging"
)

// TestMain sets up and tears down for all tests in this package
func TestMain(m *testing.M) {
	// Set log level to WARN to reduce noise during tests
	testLogger := logging.NewLogger(logging.LevelWarn, os.Stderr)
	logging.SetDefaultLogger(testLogger)

	// Run tests
	code := m.Run()

	// Restore default logger
	logging.SetDefaultLogger(logging.DefaultLogger())

	os.Exit(code)
}

func TestRandStringRunes(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"zero length", 0},
		{"single character", 1},
		{"short string", 5},
		{"medium string", 12},
		{"long string", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ids.RandStringRunes(tt.length)

			// Test length
			if len(result) != tt.length {
				t.Errorf("Expected length %d, got %d", tt.length, len(result))
			}

			// Test that result contains only valid characters
			if tt.length > 0 {
				validPattern := regexp.MustCompile("^[a-zA-Z]+$")
				if !validPattern.MatchString(result) {
					t.Errorf("Result contains invalid characters: %s", result)
				}
			}
		})
	}
}

func TestRandStringRunesUniqueness(t *testing.T) {
	// Test that multiple calls produce different results (probabilistically)
	length := 12
	results := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		result := ids.RandStringRunes(length)
		if results[result] {
			// While theoretically possible, it's extremely unlikely
			// to get duplicates with a 12-character string in 100 iterations
			t.Logf("Found duplicate: %s (this is extremely rare but not impossible)", result)
		}
		results[result] = true
	}

	// We should have close to 100 unique results
	if len(results) < 95 {
		t.Errorf("Expected at least 95 unique results, got %d", len(results))
	}
}

func TestRandStringRunesCharacterSet(t *testing.T) {
	// Test that all characters from the expected set can appear
	length := 1000 // Use a long string to increase chances of all characters appearing
	result := ids.RandStringRunes(length)

	// Count character frequencies
	charCounts := make(map[rune]int)
	for _, char := range result {
		charCounts[char]++
	}

	// Verify all characters are from the expected set
	expectedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for char := range charCounts {
		found := false
		for _, expected := range expectedChars {
			if char == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unexpected character found: %c", char)
		}
	}

	// With 1000 characters, we should see a good distribution
	// (not testing for perfect distribution, just reasonable coverage)
	if len(charCounts) < 20 {
		t.Errorf("Expected to see at least 20 different characters in 1000-char string, got %d", len(charCounts))
	}
}

func TestLetterRunes(t *testing.T) {
	// Test that the letterRunes variable is properly defined
	// Note: We can't access private variables from external package, so we'll test indirectly
	// by checking that the function generates valid characters
	result := ids.RandStringRunes(52)
	
	// Test that it contains only expected characters
	expectedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for _, char := range result {
		found := false
		for _, expected := range expectedChars {
			if char == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unexpected character found: %c", char)
		}
	}
}

func BenchmarkRandStringRunes(b *testing.B) {
	lengths := []int{1, 10, 50, 100}
	
	for _, length := range lengths {
		b.Run(fmt.Sprintf("length_%d", length), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ids.RandStringRunes(length)
			}
		})
	}
}