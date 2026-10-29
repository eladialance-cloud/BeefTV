package credits

// MicrocreditsPerCredit is the fixed-point scale used by every wallet, ledger,
// quote and task-cost amount. Six decimal places avoid binary floating point in
// billing while still allowing sub-credit charges such as 0.5 credits.
const MicrocreditsPerCredit int64 = 1_000_000

// FromWholeCredits converts an operator-facing whole-credit amount, such as a
// top-up package or registration bonus, into the internal fixed-point unit.
func FromWholeCredits(value int64) int64 {
	return value * MicrocreditsPerCredit
}
