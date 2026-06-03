package store

import (
	"strings"
	"time"
)

// ClosedPnLRecordsToTraderPositions converts exchange closed-PnL rows to API positions.
func ClosedPnLRecordsToTraderPositions(traderID, exchangeID, exchangeType string, records []ClosedPnLRecord) []*TraderPosition {
	out := make([]*TraderPosition, 0, len(records))
	nowMs := time.Now().UTC().UnixMilli()

	for i, rec := range records {
		side := strings.ToUpper(rec.Side)
		if side == "BUY" {
			side = "LONG"
		} else if side == "SELL" {
			side = "SHORT"
		} else if side == "LONG" || side == "SHORT" {
			// keep
		} else if strings.EqualFold(rec.Side, "long") {
			side = "LONG"
		} else if strings.EqualFold(rec.Side, "short") {
			side = "SHORT"
		}

		exchangePositionID := rec.ExchangeID
		if exchangePositionID == "" {
			exchangePositionID = rec.OrderID
		}

		entryQty := rec.MaxOpenQuantity
		if entryQty <= 0 {
			entryQty = rec.Quantity
		}

		out = append(out, &TraderPosition{
			ID:                 int64(i + 1),
			TraderID:           traderID,
			ExchangeID:         exchangeID,
			ExchangeType:       exchangeType,
			ExchangePositionID: exchangePositionID,
			Symbol:             rec.Symbol,
			Side:               side,
			Quantity:           rec.Quantity,
			EntryQuantity:      entryQty,
			EntryPrice:         rec.EntryPrice,
			EntryTime:          rec.EntryTime,
			ExitPrice:          rec.ExitPrice,
			ExitOrderID:        rec.OrderID,
			ExitTime:           rec.ExitTime,
			RealizedPnL:        rec.RealizedPnL,
			NetRealizedPnL:     rec.NetRealizedPnL,
			PnlRatio:           rec.PnlRatio,
			Fee:                rec.Fee,
			FundingFee:         rec.FundingFee,
			Leverage:           rec.Leverage,
			Status:             "CLOSED",
			CloseReason:        rec.CloseType,
			Source:             "okx_positions_history",
			CreatedAt:          nowMs,
			UpdatedAt:          nowMs,
		})
	}
	return out
}

// ComputeFullStatsFromPositions calculates stats from in-memory closed positions.
func ComputeFullStatsFromPositions(positions []*TraderPosition) *TraderStats {
	if len(positions) == 0 {
		return &TraderStats{}
	}
	rows := make([]TraderPosition, len(positions))
	for i, p := range positions {
		if p != nil {
			rows[i] = *p
		}
	}
	stats, _ := computeFullStatsFromRows(rows)
	return stats
}

// ComputeSymbolStatsFromPositions calculates per-symbol stats from in-memory positions.
func ComputeSymbolStatsFromPositions(positions []*TraderPosition, limit int) []SymbolStats {
	rows := make([]TraderPosition, 0, len(positions))
	for _, p := range positions {
		if p != nil {
			rows = append(rows, *p)
		}
	}
	return computeSymbolStatsFromRows(rows, limit)
}

// ComputeDirectionStatsFromPositions calculates long/short stats from in-memory positions.
func ComputeDirectionStatsFromPositions(positions []*TraderPosition) []DirectionStats {
	rows := make([]TraderPosition, 0, len(positions))
	for _, p := range positions {
		if p != nil {
			rows = append(rows, *p)
		}
	}
	return computeDirectionStatsFromRows(rows)
}

func positionDisplayPnL(pos TraderPosition) float64 {
	if pos.NetRealizedPnL != 0 {
		return pos.NetRealizedPnL
	}
	return pos.RealizedPnL
}

func computeFullStatsFromRows(positions []TraderPosition) (*TraderStats, error) {
	stats := &TraderStats{}
	if len(positions) == 0 {
		return stats, nil
	}

	var pnls []float64
	var totalWin, totalLoss float64

	for _, pos := range positions {
		pnl := positionDisplayPnL(pos)
		stats.TotalTrades++
		stats.TotalPnL += pnl
		stats.TotalFee += pos.Fee
		pnls = append(pnls, pnl)

		if pnl > 0 {
			stats.WinTrades++
			totalWin += pnl
		} else if pnl < 0 {
			stats.LossTrades++
			totalLoss += -pnl
		}
	}

	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.WinTrades) / float64(stats.TotalTrades) * 100
	}
	if totalLoss > 0 {
		stats.ProfitFactor = totalWin / totalLoss
	}
	if stats.WinTrades > 0 {
		stats.AvgWin = totalWin / float64(stats.WinTrades)
	}
	if stats.LossTrades > 0 {
		stats.AvgLoss = totalLoss / float64(stats.LossTrades)
	}
	if len(pnls) > 1 {
		stats.SharpeRatio = calculateSharpeRatioFromPnls(pnls)
	}
	if len(pnls) > 0 {
		stats.MaxDrawdownPct = calculateMaxDrawdownFromPnls(pnls)
	}
	return stats, nil
}

func computeSymbolStatsFromRows(positions []TraderPosition, limit int) []SymbolStats {
	symbolMap := make(map[string]*SymbolStats)
	symbolHoldMins := make(map[string][]float64)

	for _, pos := range positions {
		if _, ok := symbolMap[pos.Symbol]; !ok {
			symbolMap[pos.Symbol] = &SymbolStats{Symbol: pos.Symbol}
			symbolHoldMins[pos.Symbol] = []float64{}
		}
		s := symbolMap[pos.Symbol]
		pnl := positionDisplayPnL(pos)
		s.TotalTrades++
		s.TotalPnL += pnl
		if pnl > 0 {
			s.WinTrades++
		}
		if pos.ExitTime > 0 {
			holdMins := float64(pos.ExitTime-pos.EntryTime) / 60000.0
			symbolHoldMins[pos.Symbol] = append(symbolHoldMins[pos.Symbol], holdMins)
		}
	}

	var stats []SymbolStats
	for symbol, s := range symbolMap {
		if s.TotalTrades > 0 {
			s.WinRate = float64(s.WinTrades) / float64(s.TotalTrades) * 100
			s.AvgPnL = s.TotalPnL / float64(s.TotalTrades)
		}
		if mins := symbolHoldMins[symbol]; len(mins) > 0 {
			var total float64
			for _, m := range mins {
				total += m
			}
			s.AvgHoldMins = total / float64(len(mins))
		}
		stats = append(stats, *s)
	}

	for i := 0; i < len(stats)-1; i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].TotalPnL > stats[i].TotalPnL {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}
	if limit > 0 && len(stats) > limit {
		stats = stats[:limit]
	}
	return stats
}

func computeDirectionStatsFromRows(positions []TraderPosition) []DirectionStats {
	sideStats := make(map[string]*DirectionStats)
	for _, pos := range positions {
		if _, ok := sideStats[pos.Side]; !ok {
			sideStats[pos.Side] = &DirectionStats{Side: pos.Side}
		}
		s := sideStats[pos.Side]
		pnl := positionDisplayPnL(pos)
		s.TradeCount++
		s.TotalPnL += pnl
		if pnl > 0 {
			s.WinRate++
		}
	}

	var stats []DirectionStats
	for _, s := range sideStats {
		if s.TradeCount > 0 {
			s.AvgPnL = s.TotalPnL / float64(s.TradeCount)
			s.WinRate = s.WinRate / float64(s.TradeCount) * 100
		}
		stats = append(stats, *s)
	}
	return stats
}
