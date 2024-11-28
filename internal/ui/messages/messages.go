// internal/ui/messages/messages.go

package messages

type PasswordEnteredMsg string

// Dodajemy nowe typy wiadomości dla obsługi kluczy SSH
type HostKeyVerificationMsg struct {
	IP          string
	Port        string
	Fingerprint string
}

type HostKeyResponseMsg bool
