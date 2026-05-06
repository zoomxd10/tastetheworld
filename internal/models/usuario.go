// Package models define las estructuras de datos de la aplicación.
package models

// Usuario representa un usuario registrado en TastetheWorld.
// La contraseña se almacena como hash bcrypt, nunca en texto plano.
type Usuario struct {
	Nombre       string `json:"nombre"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"` // hash bcrypt, nunca texto plano
}
