package types

type WorkType int32

const (
	EcKeygen WorkType = iota + 1
	EcSigning

	EdKeygen
	EdSigning
)

var (
	WorkTypeStrings = map[WorkType]string{
		EcKeygen:  "ECDSA_KEYGEN",
		EcSigning: "ECDSA_SIGNING",

		EdKeygen:  "EDDSA_KEYGEN",
		EdSigning: "EDDSA_SIGNING",
	}
)

func (w WorkType) String() string {
	return WorkTypeStrings[w]
}

func (w WorkType) IsKeygen() bool {
	return w == EcKeygen || w == EdKeygen
}

func (w WorkType) IsSigning() bool {
	return w == EcSigning || w == EdSigning
}
