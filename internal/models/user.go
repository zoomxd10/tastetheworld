package models

// Usuario representa la estructura de datos que guardaremos en el JSON
type Usuario struct {
	Nombre   string `json:"nombre"`
	Email    string `json:"email"`
	Password string `json:"password"`
}