package types

type ReshareRequest struct {
	NewValidatorSetPubKeyBytes [][]byte `json:"new_validator_set_pub_key_bytes,omitempty"`
	OldValidatorSetPubKeyBytes [][]byte `json:"old_validator_set_pub_key_bytes,omitempty"`
}

type ReshareResult struct {
	Outcome                    OutcomeType `json:"outcome,omitempty"`
	NewValidatorSetPubKeyBytes [][]byte    `json:"new_validator_set_pub_key_bytes,omitempty"`
}
