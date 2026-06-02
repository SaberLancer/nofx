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
		PosSide:       "long",
		OpenAvgPx:     "2002.29",
		CloseAvgPx:    "2005.7084314",
		CloseTotalPos: "62.718",
		Pnl:           "214.36",
		RealizedPnl:   "88.67",
		PnlRatio:      "0.0085",
		Fee:           "-125.69",
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
	if rec.RealizedPnL != 214.36 {
		t.Fatalf("expected gross pnl 214.36, got %v", rec.RealizedPnL)
	}
	if rec.NetRealizedPnL != 88.67 {
		t.Fatalf("expected net pnl 88.67, got %v", rec.NetRealizedPnL)
	}
	if rec.Fee != 125.69 {
		t.Fatalf("expected fee 125.69, got %v", rec.Fee)
	}
	wantRatio := 214.36 / (2002.29 * 62.718 / 5)
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
