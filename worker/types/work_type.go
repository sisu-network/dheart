package types

type WorkType int32

const (
	EcKeygen WorkType = iota + 1
	EcPresign
	EcSigning

	EdKeygen
	EdPresign
	EdSigning
)

var (
	WorkTypeStrings = map[WorkType]string{
		EcKeygen:  "ECDSA_KEYGEN",
		EcPresign: "ECDSA_PRESIGN",
		EcSigning: "ECDSA_SIGNING",

		EdKeygen:  "EDDSA_KEYGEN",
		EdPresign: "EDDSA_PRESIGN",
		EdSigning: "EDDSA_SIGNING",
	}
)

func (w WorkType) String() string {
	return WorkTypeStrings[w]
}

func (w WorkType) IsKeygen() bool {
	return w == EcKeygen || w == EdKeygen
}

func (w WorkType) IsPresign() bool {
	return w == EcPresign || w == EdPresign
}

func (w WorkType) IsSigning() bool {
	return w == EcSigning || w == EdSigning
}
