package store

// EffectiveMaxPositions returns the enforced max simultaneous positions.
// When max_positions is 0 or unset, defaults to the effective candidate coin count.
// The result never exceeds the candidate coin count.
func (c *StrategyConfig) EffectiveMaxPositions() int {
	coinCount := c.getEffectiveCoinCount()
	max := c.RiskControl.MaxPositions
	if max <= 0 {
		return coinCount
	}
	if max > coinCount {
		return coinCount
	}
	return max
}
