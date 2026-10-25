package push

// Config is the VAPID identity the server signs its sends with. PublicKey and
// PrivateKey are base64url strings from go run ./cmd/vapidkeys; Subject is a
// bare email address or an https URL (webpush-go adds "mailto:" itself).
type Config struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}
