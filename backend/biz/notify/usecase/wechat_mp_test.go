package usecase

import "testing"

func TestExtractScene(t *testing.T) {
	cases := []struct {
		name        string
		eventKey    string
		isSubscribe bool
		want        string
	}{
		{
			name:        "subscribe with qrscene prefix",
			eventKey:    "qrscene_devloom:8VYkSx4q9rP2mN6a",
			isSubscribe: true,
			want:        "devloom:8VYkSx4q9rP2mN6a",
		},
		{
			name:        "subscribe with empty event key",
			eventKey:    "",
			isSubscribe: true,
			want:        "",
		},
		{
			name:        "subscribe with prefix only",
			eventKey:    "qrscene_",
			isSubscribe: true,
			want:        "",
		},
		{
			name:        "subscribe without qrscene prefix is returned as-is",
			eventKey:    "devloom:abc",
			isSubscribe: true,
			want:        "devloom:abc",
		},
		{
			name:        "SCAN event without prefix",
			eventKey:    "devloom:8VYkSx4q9rP2mN6a",
			isSubscribe: false,
			want:        "devloom:8VYkSx4q9rP2mN6a",
		},
		{
			name:        "SCAN event preserves qrscene prefix (only subscribe strips it)",
			eventKey:    "qrscene_devloom:abc",
			isSubscribe: false,
			want:        "qrscene_devloom:abc",
		},
		{
			name:        "SCAN event with empty key",
			eventKey:    "",
			isSubscribe: false,
			want:        "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractScene(tc.eventKey, tc.isSubscribe)
			if got != tc.want {
				t.Errorf("ExtractScene(%q, %v) = %q, want %q", tc.eventKey, tc.isSubscribe, got, tc.want)
			}
		})
	}
}
