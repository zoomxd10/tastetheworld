// Package storage — sesiones en memoria.
// Las sesiones se guardan en un mapa protegido por sync.RWMutex.
// Al reiniciar el servidor las sesiones desaparecen (comportamiento correcto
// para una aplicación sin base de datos persistente de sesiones).
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Sesion almacena los datos de un usuario autenticado.
type Sesion struct {
	NombreUsuario string
	Email         string
	Expira        time.Time
}

// SesionStore gestiona las sesiones activas en memoria.
type SesionStore struct {
	mu      sync.RWMutex
	sesiones map[string]Sesion
}

// NuevoSesionStore crea un SesionStore listo para usar.
func NuevoSesionStore() *SesionStore {
	return &SesionStore{sesiones: make(map[string]Sesion)}
}

// Crear genera un token de sesión aleatorio, guarda la sesión y devuelve el token.
func (s *SesionStore) Crear(nombre, email string) (string, error) {
	// Generar token de 32 bytes aleatorios (256 bits).
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sesiones[token] = Sesion{
		NombreUsuario: nombre,
		Email:         email,
		Expira:        time.Now().Add(24 * time.Hour),
	}
	return token, nil
}

// Obtener devuelve la sesión asociada al token, o false si no existe o ha expirado.
func (s *SesionStore) Obtener(token string) (Sesion, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sesion, ok := s.sesiones[token]
	if !ok || time.Now().After(sesion.Expira) {
		return Sesion{}, false
	}
	return sesion, true
}

// Eliminar borra la sesión asociada al token (logout).
func (s *SesionStore) Eliminar(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sesiones, token)
}
