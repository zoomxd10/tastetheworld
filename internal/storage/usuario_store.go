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

// leerSinBloqueo lee el archivo JSON sin adquirir el mutex.
// Solo debe llamarse desde métodos que ya tienen el lock adquirido.
func (s *UsuarioStore) leerSinBloqueo() ([]models.Usuario, error) {
	datos, err := os.ReadFile(s.filepath)
	if err != nil {
		if os.IsNotExist(err) {
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

// escribirSinBloqueo serializa el slice y lo guarda de forma atómica (tmp + rename).
// Solo debe llamarse desde métodos que ya tienen el lock adquirido.
func (s *UsuarioStore) escribirSinBloqueo(usuarios []models.Usuario) error {
	datosFinales, err := json.MarshalIndent(usuarios, "", "  ")
	if err != nil {
		return fmt.Errorf("storage: serializar JSON: %w", err)
	}
	tmp := s.filepath + ".tmp"
	if err := os.WriteFile(tmp, datosFinales, 0644); err != nil {
		return fmt.Errorf("storage: escribir archivo temporal: %w", err)
	}
	if err := os.Rename(tmp, s.filepath); err != nil {
		return fmt.Errorf("storage: renombrar archivo: %w", err)
	}
	return nil
}

// LeerTodos devuelve todos los usuarios guardados en el archivo JSON.
// Devuelve un slice vacío (no nil) si el archivo no existe todavía.
func (s *UsuarioStore) LeerTodos() ([]models.Usuario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leerSinBloqueo()
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
// Devuelve error si el guardado falla, en lugar de ignorarlo silenciosamente.
func (s *UsuarioStore) Guardar(u models.Usuario) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	usuarios, err := s.leerSinBloqueo()
	if err != nil {
		return err
	}
	usuarios = append(usuarios, u)
	return s.escribirSinBloqueo(usuarios)
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

// Actualizar modifica el nombre del usuario identificado por email.
// Devuelve (true, nil) si se actualizó, (false, nil) si no existe.
func (s *UsuarioStore) Actualizar(email, nuevoNombre string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	usuarios, err := s.leerSinBloqueo()
	if err != nil {
		return false, err
	}
	encontrado := false
	for i, u := range usuarios {
		if u.Email == email {
			usuarios[i].Nombre = nuevoNombre
			encontrado = true
			break
		}
	}
	if !encontrado {
		return false, nil
	}
	return true, s.escribirSinBloqueo(usuarios)
}

// Eliminar borra el usuario identificado por email.
// Devuelve (true, nil) si se eliminó, (false, nil) si no existía.
func (s *UsuarioStore) Eliminar(email string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	usuarios, err := s.leerSinBloqueo()
	if err != nil {
		return false, err
	}
	nuevos := make([]models.Usuario, 0, len(usuarios))
	encontrado := false
	for _, u := range usuarios {
		if u.Email == email {
			encontrado = true
			continue // omitir: este es el que se borra
		}
		nuevos = append(nuevos, u)
	}
	if !encontrado {
		return false, nil
	}
	return true, s.escribirSinBloqueo(nuevos)
}
