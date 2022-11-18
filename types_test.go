package lambdaext_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	lambdaext "github.com/zakharovvi/aws-lambda-extensions"
)

func TestDurationMs_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    lambdaext.DurationMs
		json    []byte
		wantErr bool
	}{
		{
			"float",
			lambdaext.DurationMs(90100 * time.Microsecond),
			[]byte("90.1"),
			false,
		},
		{
			"int",
			lambdaext.DurationMs(694 * time.Millisecond),
			[]byte("694"),
			false,
		},
		{
			"unsupported",
			lambdaext.DurationMs(0),
			[]byte(`"10s"`),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := lambdaext.DurationMs(0)
			if err := json.Unmarshal(tt.json, &got); (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDurationMs_MarshalJSON(t *testing.T) {
	d := lambdaext.DurationMs(1*time.Hour + 2*time.Minute + 23*time.Second + 387*time.Millisecond)
	got, err := json.Marshal(d)
	require.NoError(t, err)
	require.Equal(t, `"1h2m23.387s"`, string(got))
}
