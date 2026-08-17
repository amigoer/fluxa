package types

type OTPPurpose string

const (
	OTPPurposeRegister OTPPurpose = "register"
	OTPPurposeLogin    OTPPurpose = "login"
)
