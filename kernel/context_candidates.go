package kernel

import "nofx/market"

// RestrictCandidatesToPositions keeps only candidate entries for symbols with open positions.
// Position symbols missing from CandidateCoins are added so market fetch and prompts still cover them.
func RestrictCandidatesToPositions(ctx *Context) {
	if ctx == nil || len(ctx.Positions) == 0 {
		return
	}
	posSet := make(map[string]struct{}, len(ctx.Positions))
	for _, pos := range ctx.Positions {
		sym := market.Normalize(pos.Symbol)
		if sym != "" {
			posSet[sym] = struct{}{}
		}
	}
	if len(posSet) == 0 {
		return
	}

	bySymbol := make(map[string]CandidateCoin, len(ctx.CandidateCoins))
	for _, c := range ctx.CandidateCoins {
		sym := market.Normalize(c.Symbol)
		if sym != "" {
			bySymbol[sym] = c
		}
	}

	filtered := make([]CandidateCoin, 0, len(posSet))
	for sym := range posSet {
		if c, ok := bySymbol[sym]; ok {
			filtered = append(filtered, c)
		} else {
			filtered = append(filtered, CandidateCoin{Symbol: sym, Sources: []string{"position"}})
		}
	}
	ctx.CandidateCoins = filtered

	if ctx.MarketDataMap != nil {
		for sym := range ctx.MarketDataMap {
			norm := market.Normalize(sym)
			if _, ok := posSet[norm]; !ok {
				delete(ctx.MarketDataMap, sym)
			}
		}
	}
	if ctx.MarketDataFailures != nil {
		for sym := range ctx.MarketDataFailures {
			norm := market.Normalize(sym)
			if _, ok := posSet[norm]; !ok {
				delete(ctx.MarketDataFailures, sym)
			}
		}
	}
	if ctx.QuantDataMap != nil {
		for sym := range ctx.QuantDataMap {
			norm := market.Normalize(sym)
			if _, ok := posSet[norm]; !ok {
				delete(ctx.QuantDataMap, sym)
			}
		}
	}
}
