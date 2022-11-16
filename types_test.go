package lambdaext

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDurationMs_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    DurationMs
		json    []byte
		wantErr bool
	}{
		{
			"float",
			DurationMs(90100 * time.Microsecond),
			[]byte("90.1"),
			false,
		},
		{
			"int",
			DurationMs(694 * time.Millisecond),
			[]byte("694"),
			false,
		},
		{
			"unsupported",
			DurationMs(0),
			[]byte(`"10s"`),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DurationMs(0)
			if err := json.Unmarshal(tt.json, &got); (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("json.Unmarshal() got = %#v, want = %#v", got, tt.want)
			}
		})
	}
}
