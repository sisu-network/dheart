package types

type WorkType int32

const (
	EcdsaKeygen WorkType = iota + 1
	EcdsaPresign
	EcdsaSigning

	EddsaKeygen
	EddsaPresign
	EddsaSigning
)

var (
	WorkTypeStrings = map[WorkType]string{
		EcdsaKeygen:  "ECDSA_KEYGEN",
		EcdsaPresign: "ECDSA_PRESIGN",
		EcdsaSigning: "ECDSA_SIGNING",

		EddsaKeygen:  "EDDSA_KEYGEN",
		EddsaPresign: "EDDSA_PRESIGN",
		EddsaSigning: "EDDSA_SIGNING",
	}
)

func (w WorkType) String() string {
	return WorkTypeStrings[w]
}

func (w WorkType) IsKeygen() bool {
	return w == EcdsaKeygen || w == EddsaKeygen
}

func (w WorkType) IsPresign() bool {
	return w == EcdsaPresign || w == EddsaPresign
}

func (w WorkType) IsSigning() bool {
	return w == EcdsaSigning || w == EddsaSigning
}
