package usage

import (
	"testing"
	"time"
)

func TestOutputTokensCountsStreamedTextWhenUsageIsMissing(t *testing.T) {
	raw := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
	if got := OutputTokens(raw); got < 1 {
		t.Fatalf("output tokens %d", got)
	}
}

func TestTTFTMillisOmitsAnUnmeasuredDelay(t *testing.T) {
	if TtftMillis(0) != nil {
		t.Fatal("zero ttft must stay unset")
	}
	got := TtftMillis(1500 * time.Millisecond)
	if got == nil || *got != 1500 {
		t.Fatalf("ttft %v", got)
	}
}
