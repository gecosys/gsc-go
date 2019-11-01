package socket

type GEConnection struct {
	ID        string
	Token     string
	AliasName string
	Ver       string
}

// GEClientInfo is information returned from GSCHub
type GEClientInfo struct {
	ID    string
	Token string
}

type GEResponse struct {
	ReturnCode int
	Data       string
	Timestamp  int64
}
