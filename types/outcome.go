package types

type OutcomeType int

const (
	OutcomeSuccess OutcomeType = iota
	OutcomeFailure
	OutcometNotSelected // not participate in the round
)
