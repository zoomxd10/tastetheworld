// Package storage gestiona la persistencia de datos en archivo JSON.
// Protege las escrituras concurrentes con un sync.Mutex para evitar
// condiciones de carrera (race conditions).
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"tastetheworld/internal/models"
)

// UsuarioStore encapsula el acceso al archivo JSON de usuarios.
// El mutex garantiza que solo una goroutine a la vez puede leer o escribir.
type UsuarioStore struct {
	mu       sync.Mutex
	filepath string
}

// NuevoUsuarioStore crea una instancia de UsuarioStore apuntando al archivo dado.
func NuevoUsuarioStore(filepath string) *UsuarioStore {
	return &UsuarioStore{filepath: filepath}
}

// LeerTodos devuelve todos los usuarios guardados en el archivo JSON.
// Devuelve un slice vacío (no nil) si el archivo no existe todavía.
func (s *UsuarioStore) LeerTodos() ([]models.Usuario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	datos, err := os.ReadFile(s.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// El archivo aún no existe; devolvemos slice vacío sin error.
			return []models.Usuario{}, nil
		}
		return nil, fmt.Errorf("storage: leer archivo: %w", err)
	}

	var usuarios []models.Usuario
	if err := json.Unmarshal(datos, &usuarios); err != nil {
		return nil, fmt.Errorf("storage: parsear JSON: %w", err)
	}
	return usuarios, nil
}

// ExisteEmail comprueba si ya hay un usuario registrado con ese email.
func (s *UsuarioStore) ExisteEmail(email string) (bool, error) {
	usuarios, err := s.LeerTodos()
	if err != nil {
		return false, err
	}
	for _, u := range usuarios {
		if u.Email == email {
			return true, nil
		}
	}
	return false, nil
}

// Guardar añade un nuevo usuario al archivo JSON de forma segura.
// Si el guardado falla devuelve error en lugar de ignorarlo silenciosamente.
func (s *UsuarioStore) Guardar(u models.Usuario) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Leer estado actual dentro del bloqueo para evitar race condition.
	var usuarios []models.Usuario
	datos, err := os.ReadFile(s.filepath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: leer antes de guardar: %w", err)
	}
	if len(datos) > 0 {
		if err := json.Unmarshal(datos, &usuarios); err != nil {
			return fmt.Errorf("storage: parsear JSON existente: %w", err)
		}
	}

	usuarios = append(usuarios, u)

	datosFinales, err := json.MarshalIndent(usuarios, "", "  ")
	if err != nil {
		return fmt.Errorf("storage: serializar JSON: %w", err)
	}

	// Escribir en archivo temporal y renombrar para escritura atómica.
	tmpFile := s.filepath + ".tmp"
	if err := os.WriteFile(tmpFile, datosFinales, 0644); err != nil {
		return fmt.Errorf("storage: escribir archivo temporal: %w", err)
	}
	if err := os.Rename(tmpFile, s.filepath); err != nil {
		return fmt.Errorf("storage: renombrar archivo: %w", err)
	}

	return nil
}

// BuscarPorEmail devuelve el usuario con ese email, o nil si no existe.
func (s *UsuarioStore) BuscarPorEmail(email string) (*models.Usuario, error) {
	usuarios, err := s.LeerTodos()
	if err != nil {
		return nil, err
	}
	for _, u := range usuarios {
		if u.Email == email {
			copia := u // evitar devolver puntero a variable de bucle
			return &copia, nil
		}
	}
	return nil, nil
}
