package market

import "testing"

func TestMapTimeframeToOKXBar(t *testing.T) {
	bar, err := mapTimeframeToOKXBar("3m")
	if err != nil || bar != "3m" {
		t.Fatalf("expected 3m, got %q err=%v", bar, err)
	}
	bar, err = mapTimeframeToOKXBar("1h")
	if err != nil || bar != "1H" {
		t.Fatalf("expected 1H, got %q err=%v", bar, err)
	}
}

func TestParseOKXCandlesJSON(t *testing.T) {
	body := []byte(`{"code":"0","data":[["3000","101","105","99","103","12"],["2000","100","104","98","101","10"]]}`)
	klines, err := parseOKXCandlesJSON(body)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(klines) != 2 {
		t.Fatalf("expected 2 klines, got %d", len(klines))
	}
	if klines[0].OpenTime != 2000 || klines[1].OpenTime != 3000 {
		t.Fatalf("expected ascending order, got %d then %d", klines[0].OpenTime, klines[1].OpenTime)
	}
	if klines[1].Close != 103 {
		t.Fatalf("unexpected close: %v", klines[1].Close)
	}
}
