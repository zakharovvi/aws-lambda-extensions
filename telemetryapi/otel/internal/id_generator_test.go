package internal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel/internal"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
)

func TestIDGenerator_NewIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	randomGen := xray.NewIDGenerator()
	gen := &internal.IDGenerator{
		Gen: randomGen,
	}

	wantTraceID, wantSpanID := randomGen.NewIDs(ctx)
	gen.SetNext(wantTraceID, wantSpanID)

	// after setting returned IDs should be equal
	gotTraceID, gotSpanID := gen.NewIDs(ctx)
	require.Equal(t, wantTraceID, gotTraceID)
	require.Equal(t, wantSpanID, gotSpanID)

	// after having returned, new IDs should be generated
	gotTraceID, gotSpanID = gen.NewIDs(ctx)
	require.NotEqual(t, wantTraceID, gotTraceID)
	require.NotEqual(t, wantSpanID, gotSpanID)
}

func TestIDGenerator_NewSpanID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	randomGen := xray.NewIDGenerator()
	gen := &internal.IDGenerator{
		Gen: randomGen,
	}

	traceID, wantSpanID := randomGen.NewIDs(ctx)
	gen.SetNext(traceID, wantSpanID)

	// after setting returned ID should be equal
	gotSpanID := gen.NewSpanID(ctx, traceID)
	require.Equal(t, wantSpanID, gotSpanID)

	// after having returned, new ID should be generated
	gotSpanID = gen.NewSpanID(ctx, traceID)
	require.NotEqual(t, wantSpanID, gotSpanID)
}
