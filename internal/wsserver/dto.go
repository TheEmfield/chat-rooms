package wsserver

type wsMessage struct {
	Type      string       `json:"type,omitempty"`
	IPAddress string       `json:"address"`
	Message   string       `json:"message"`
	Time      string       `json:"time"`
	Messages  []*wsMessage `json:"messages,omitempty"`
}
