package types

type WorkType int32

const (
	ECDSA_KEYGEN = iota
	ECDSA_PRESIGN
	ECDSA_SIGNING

	EDDSA_KEYGEN
	EDDSA_PRESIGN
	EDDSA_SIGNING
)

var (
	WorkTypeStrings = map[WorkType]string{
		ECDSA_KEYGEN:  "ECDSA_KEYGEN",
		ECDSA_PRESIGN: "ECDSA_PRESIGN",
		ECDSA_SIGNING: "ECDSA_SIGNING",

		EDDSA_KEYGEN:  "EDDSA_KEYGEN",
		EDDSA_PRESIGN: "EDDSA_PRESIGN",
		EDDSA_SIGNING: "EDDSA_SIGNING",
	}
)

func (w WorkType) String() string {
	return WorkTypeStrings[w]
}
