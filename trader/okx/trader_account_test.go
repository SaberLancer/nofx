package okx

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseOKXPositionsHistoryData_ArrayPayload(t *testing.T) {
	body := []byte(`[{"instId":"ETH-USDT-SWAP","posSide":"long","openAvgPx":"2000","closeAvgPx":"2010","closeTotalPos":"10","realizedPnl":"5","fee":"-0.1","fundingFee":"0","lever":"5","cTime":"1700000000000","uTime":"1700003600000","type":"1","posId":"p1"}]`)
	rows, err := parseOKXPositionsHistoryData(body)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(rows) != 1 || rows[0].PosSide != "long" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestGetClosedPnLPathUsesBeforeNotAfter(t *testing.T) {
	start := time.UnixMilli(1700000000000)
	path := buildOKXPositionsHistoryPath(start, 50, "")
	if strings.Contains(path, "after=") {
		t.Fatalf("should not use after= for forward sync: %s", path)
	}
	if !strings.Contains(path, "before=1700000000000") {
		t.Fatalf("expected before= in path: %s", path)
	}
	noFilter := buildOKXPositionsHistoryPath(time.Time{}, 100, "")
	if strings.Contains(noFilter, "before=") || strings.Contains(noFilter, "after=") {
		t.Fatalf("first sync should not add pagination: %s", noFilter)
	}
}

func TestGetClosedPnLPathPaginationUsesAfter(t *testing.T) {
	path := buildOKXPositionsHistoryPath(time.Time{}, 100, "1700003600000")
	if !strings.Contains(path, "after=1700003600000") {
		t.Fatalf("expected after= in pagination path: %s", path)
	}
	if strings.Contains(path, "before=") {
		t.Fatalf("pagination page should not use before=: %s", path)
	}
}

func TestParseOKXPositionsHistoryData_PnlFields(t *testing.T) {
	body := []byte(`[{"instId":"ETH-USDT-SWAP","posSide":"long","openAvgPx":"2002.29","closeAvgPx":"2005.7084314","closeTotalPos":"627.18","pnl":"214.36","realizedPnl":"88.67","pnlRatio":"0.0085","fee":"-125.69","fundingFee":"0","lever":"5","cTime":"1700000000000","uTime":"1700003600000","type":"2","posId":"p1"}]`)
	rows, err := parseOKXPositionsHistoryData(body)
	if err != nil || len(rows) != 1 {
		t.Fatalf("parse failed: rows=%v err=%v", rows, err)
	}
	if rows[0].Pnl != "214.36" || rows[0].RealizedPnl != "88.67" {
		t.Fatalf("unexpected pnl fields: %+v", rows[0])
	}
}

func TestOKXPositionHistoryRowToRecord_UsesGrossPnl(t *testing.T) {
	tr := &OKXTrader{
		instrumentsCache: map[string]*OKXInstrument{
			"ETH-USDT-SWAP": {CtVal: 1},
		},
		instrumentsCacheTime: time.Now(),
	}
	row := okxPositionHistoryRow{
		InstID:        "ETH-USDT-SWAP",
		PosSide:       "short",
		OpenAvgPx:     "1898.27",
		CloseAvgPx:    "1878.27",
		OpenMaxPos:    "15.806",
		CloseTotalPos: "15.806",
		Pnl:           "286.41",
		RealizedPnl:   "280.00",
		PnlRatio:      "0.0477",
		Fee:           "-6.41",
		FundingFee:    "0",
		Lever:         "5",
		CTime:         "1700000000000",
		UTime:         "1700003600000",
		Type:          "2",
		PosId:         "p1",
	}
	rec, ok := tr.okxPositionHistoryRowToRecord(row)
	if !ok {
		t.Fatal("expected valid record")
	}
	if rec.RealizedPnL != 286.41 {
		t.Fatalf("expected gross pnl 286.41, got %v", rec.RealizedPnL)
	}
	if rec.NetRealizedPnL != 280.00 {
		t.Fatalf("expected net pnl 280.00, got %v", rec.NetRealizedPnL)
	}
	if rec.MaxOpenQuantity != 15.806 {
		t.Fatalf("expected max open qty 15.806, got %v", rec.MaxOpenQuantity)
	}
	if rec.Quantity != 15.806 {
		t.Fatalf("expected close qty 15.806, got %v", rec.Quantity)
	}
	if rec.Fee != 6.41 {
		t.Fatalf("expected fee 6.41, got %v", rec.Fee)
	}
	wantRatio := 280.00 / (1898.27 * 15.806 / 5)
	if math.Abs(rec.PnlRatio-wantRatio) > 0.0001 {
		t.Fatalf("expected gross roi ~%v, got %v", wantRatio, rec.PnlRatio)
	}
}

func TestParseOKXPositionsHistoryData_EnvelopePayload(t *testing.T) {
	body := []byte(`{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","posSide":"short","openAvgPx":"70000","closeAvgPx":"69000","closeTotalPos":"1","realizedPnl":"10","cTime":"1700000000000","uTime":"1700003600000","posId":"p2"}]}`)
	rows, err := parseOKXPositionsHistoryData(body)
	if err != nil || len(rows) != 1 {
		t.Fatalf("parse envelope failed: rows=%v err=%v", rows, err)
	}
}
