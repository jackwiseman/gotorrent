package models

// func TestNewMagnet(t *testing.T) {
// 	testCases := []struct {
// 		testLink     string
// 		expected     Magnet
// 		expectsError bool
// 	}{
// 		{
// 			testLink:     "magnet:?xt=urn:btih:fffffffffffffffffffffffffffffffffffffff&dn=example&tr=https://example.com",
// 			expected:     Magnet{DisplayName: "example", Trackers: []string{"https://example.com"}, ExactTopic: "btih:fffffffffffffffffffffffffffffffffffffff"},
// 			expectsError: false,
// 		},
// 	}
//
// 	for _, tc := range testCases {
// 		magnet, err := NewMagnet(tc.testLink)
//
// 		if tc.expectsError {
// 			if err == nil {
// 				t.Errorf("Expected error but got nil")
// 			}
// 		} else {
// 			if err != nil {
// 				t.Errorf("Expected no error but got: %v", err)
// 			}
//
// 			// Compare the returned Magnet with the expected Magnet
// 			if !reflect.DeepEqual(magnet, &tc.expected) {
// 				t.Errorf("Returned Magnet %+v does not match expected %+v", magnet, tc.expected)
// 			}
// 		}
// 	}
// }
