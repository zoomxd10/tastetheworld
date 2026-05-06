// Punto de entrada de la aplicación TastetheWorld.
// Se encarga de:
//   1. Inicializar el logger.
//   2. Parsear y cachear todas las plantillas HTML al arranque.
//   3. Inyectar las dependencias en la struct Aplicacion.
//   4. Registrar las rutas y arrancar el servidor.
package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"tastetheworld/internal/handlers"
	"tastetheworld/internal/storage"
)

func main() {
	// --- 1. LOGGER ---
	logger := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)

	// --- 2. CACHÉ DE PLANTILLAS ---
	// Las plantillas se parsean una sola vez al arrancar, no en cada petición.
	plantillas, err := cargarPlantillas("./ui/html")
	if err != nil {
		logger.Fatalf("Error al cargar plantillas: %v", err)
	}

	// --- 3. CAPA DE DATOS ---
	usuarioStore := storage.NuevoUsuarioStore("./data/usuarios.json")
	sesionStore := storage.NuevoSesionStore()

	// --- 4. INYECCIÓN DE DEPENDENCIAS ---
	app := &handlers.Aplicacion{
		Log:          logger,
		Plantillas:   plantillas,
		UsuarioStore: usuarioStore,
		SesionStore:  sesionStore,
	}

	// --- 5. RUTAS ---
	mux := http.NewServeMux()

	// Archivos estáticos (CSS, imágenes…)
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	// Páginas de la aplicación
	mux.HandleFunc("/", app.Home)
	mux.HandleFunc("/ranking", app.Ranking)
	mux.HandleFunc("/resenas", app.Resenas)
	mux.HandleFunc("/registro", app.Registro)
	mux.HandleFunc("/login", app.Login)
	mux.HandleFunc("/logout", app.Logout)

	// --- 6. ARRANQUE DEL SERVIDOR ---
	addr := ":8000"
	logger.Printf("Servidor arrancando en http://localhost%s", addr)
	err = http.ListenAndServe(addr, mux)
	logger.Fatal(err)
}

// cargarPlantillas parsea todos los archivos HTML de ui/html y los almacena
// en un mapa nombre→*template.Template. Cada página extiende base.html.
func cargarPlantillas(dir string) (map[string]*template.Template, error) {
	cache := make(map[string]*template.Template)

	paginas, err := filepath.Glob(filepath.Join(dir, "*.html"))
	if err != nil {
		return nil, err
	}

	base := filepath.Join(dir, "base.html")

	for _, pagina := range paginas {
		nombre := filepath.Base(pagina)
		if nombre == "base.html" {
			continue
		}
		ts, err := template.ParseFiles(base, pagina)
		if err != nil {
			return nil, err
		}
		cache[nombre] = ts
	}

	return cache, nil
}
