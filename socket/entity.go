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

type GEHostHub struct {
	Host  string
	Token string
	ID    string
}

type GERegisterResponse struct {
	StatusCode int
	ReturnCode int
	Data       GEHostHub
	Timestamp  int64
}

type GERenameResposne struct {
	StatusCode int
	ReturnCode int
	Data       interface{}
	Timestamp  int64
}
