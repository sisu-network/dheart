package types

type ReshareResult struct {
	Outcome                    OutcomeType `json:"outcome,omitempty"`
	NewValidatorSetPubKeyBytes [][]byte    `json:"new_validator_set_pub_key_bytes,omitempty"`
}
