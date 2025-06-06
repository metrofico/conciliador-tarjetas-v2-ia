package reports_models

type ServerInfo struct {
	Ip       string `json:"ip"`
	Port     string `json:"port"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
