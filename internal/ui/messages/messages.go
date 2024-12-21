package messages

type PasswordEnteredMsg string

type HostKeyVerificationMsg struct {
	IP          string
	Port        string
	Fingerprint string
}

type ApiKeyEnteredMsg struct {
	Key       string
	LocalMode bool
}

type HostKeyResponseMsg bool
type ReloadAppMsg struct{}
type ShellExitedMsg struct{}
type SessionEndedMsg struct{}
type AutoCloseMsg struct{}
