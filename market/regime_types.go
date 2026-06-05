package market

// MultiTFRegimeVerdict is trend, range, or inconclusive (gray zone).
type MultiTFRegimeVerdict string

const (
	RegimeVerdictTrend        MultiTFRegimeVerdict = "trend"
	RegimeVerdictRange        MultiTFRegimeVerdict = "range"
	RegimeVerdictInconclusive MultiTFRegimeVerdict = "inconclusive"
)

// MultiTFRegimeSnapshot is the combined 1H + 15m regime read for strategy switching.
type MultiTFRegimeSnapshot struct {
	Verdict       MultiTFRegimeVerdict
	IsOscillating bool
	Decisive      bool
	ADX1H         float64
	ADX15m        float64
	BBW15mPct     float64
	EMABias1H     string
	Reason        string
}
