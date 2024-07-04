package utils

import (
	"reflect"
	"testing"
)

// Return a new, random 32-bit integer
// func TestGetTransactionID(t *testing.T) {}

func TestSetBit(t *testing.T) {
	// Test cases with initial data and expected results
	testCases := []struct {
		initialData  []byte
		pos          int
		expected     []byte
		expectsError bool
	}{
		{
			// [ 0 0 0 0 0 0 0 0 ]  -> [ 0 0 0 1 0 0 0 0 ]
			initialData:  []byte{0},
			pos:          3,
			expected:     []byte{16},
			expectsError: false,
		},
		{
			initialData:  []byte{0},
			pos:          8,
			expected:     nil,
			expectsError: true,
		},
	}

	for _, tc := range testCases {
		// Make a copy of the initial data to avoid modifying the original slice
		dataCopy := make([]byte, len(tc.initialData))
		copy(dataCopy, tc.initialData)

		// Call the SetBit function with the copy of initial data
		err := SetBit(&dataCopy, tc.pos)
		if err != nil && !tc.expectsError {
			// Check if the modified data matches the expected result
			if !reflect.DeepEqual(dataCopy, tc.expected) {
				t.Errorf("SetBit failed. Expected %v, got %v", tc.expected, dataCopy)
			}
		}
	}
}

func TestUnsetBit(t *testing.T) {
	// Test cases with initial data and expected results
	testCases := []struct {
		initialData  []byte
		pos          int
		expected     []byte
		expectsError bool
	}{
		{
			// [ 0 0 0 0 0 0 0 0 ]  -> [ 0 0 0 1 0 0 0 0 ]
			initialData:  []byte{16},
			pos:          3,
			expected:     []byte{0},
			expectsError: false,
		},
		{
			initialData:  []byte{0},
			pos:          8,
			expected:     nil,
			expectsError: true,
		},
	}

	for _, tc := range testCases {
		// Make a copy of the initial data to avoid modifying the original slice
		dataCopy := make([]byte, len(tc.initialData))
		copy(dataCopy, tc.initialData)

		// Call the SetBit function with the copy of initial data
		err := UnsetBit(&dataCopy, tc.pos)
		if err != nil && !tc.expectsError {
			// Check if the modified data matches the expected result
			if !reflect.DeepEqual(dataCopy, tc.expected) {
				t.Errorf("SetBit failed. Expected %v, got %v", tc.expected, dataCopy)
			}
		}
	}
}

func TestBitIsSet(t *testing.T) {
	// Test cases with initial data and expected results
	testCases := []struct {
		initialData  []byte
		pos          int
		expected     bool
		expectsError bool
	}{
		{
			// [ 0 0 0 0 0 0 0 0 ]  -> [ 0 0 0 1 0 0 0 0 ]
			initialData:  []byte{16},
			pos:          3,
			expected:     true,
			expectsError: false,
		},
		{
			initialData:  []byte{0},
			pos:          8,
			expected:     false,
			expectsError: true,
		},
		{
			// [ 0 0 0 0 0 0 0 0 ]  -> [ 0 0 0 1 0 0 0 0 ]
			initialData:  []byte{16},
			pos:          5,
			expected:     false,
			expectsError: false,
		},
	}

	for _, tc := range testCases {
		// Make a copy of the initial data to avoid modifying the original slice
		dataCopy := make([]byte, len(tc.initialData))
		copy(dataCopy, tc.initialData)

		// Call the SetBit function with the copy of initial data
		isSet, err := BitIsSet(dataCopy, tc.pos)
		if err != nil && !tc.expectsError {
			t.Errorf(err.Error())
		}

		if isSet != tc.expected {
			t.Errorf("SetBit failed. Expected %v, got %v", tc.expected, isSet)
		}
	}
}
