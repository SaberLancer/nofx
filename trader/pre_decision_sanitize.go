package trader

import (
	"fmt"
	"nofx/market"
	"nofx/store"
)

type tickConflictSeverity int

const (
	tickConflictNone tickConflictSeverity = iota
	tickConflictReduce
	tickConflictBlock
)

// evaluateTickConflict classifies tick flow vs intended open direction.
// Severe conflict → block; mild pressure mismatch → reduce size.
func evaluateTickConflict(action string, signal market.TrendSignal, cfg store.PreDecisionConfig) (tickConflictSeverity, string) {
	blockGap := cfg.EffectiveTickConflictBlockGap()
	reduceGap := cfg.EffectiveTickConflictReduceGap()
	if reduceGap > blockGap {
		reduceGap = blockGap
	}

	minOpp := cfg.MinSellPressure
	if cfg.MinBuyPressure > minOpp {
		minOpp = cfg.MinBuyPressure
	}
	if minOpp < 0.60 {
		minOpp = 0.60
	}

	switch action {
	case "open_long":
		if signal.Direction == market.TrendShort {
			return tickConflictBlock, fmt.Sprintf("tick short sell=%.1f%% momentum=%+.3f%%",
				signal.SellPressure*100, signal.MomentumPct)
		}
		gap := signal.SellPressure - signal.BuyPressure
		if signal.SellPressure >= minOpp && gap >= blockGap {
			return tickConflictBlock, fmt.Sprintf("tick sell pressure %.1f%% > buy %.1f%% (gap %.0f%%)",
				signal.SellPressure*100, signal.BuyPressure*100, gap*100)
		}
		if signal.SellPressure >= minOpp && gap >= reduceGap {
			return tickConflictReduce, fmt.Sprintf("mild sell skew %.1f%% vs buy %.1f%%",
				signal.SellPressure*100, signal.BuyPressure*100)
		}
	case "open_short":
		if signal.Direction == market.TrendLong {
			return tickConflictBlock, fmt.Sprintf("tick long buy=%.1f%% momentum=%+.3f%%",
				signal.BuyPressure*100, signal.MomentumPct)
		}
		gap := signal.BuyPressure - signal.SellPressure
		if signal.BuyPressure >= minOpp && gap >= blockGap {
			return tickConflictBlock, fmt.Sprintf("tick buy pressure %.1f%% > sell %.1f%% (gap %.0f%%)",
				signal.BuyPressure*100, signal.SellPressure*100, gap*100)
		}
		if signal.BuyPressure >= minOpp && gap >= reduceGap {
			return tickConflictReduce, fmt.Sprintf("mild buy skew %.1f%% vs sell %.1f%%",
				signal.BuyPressure*100, signal.SellPressure*100)
		}
	}
	return tickConflictNone, ""
}
