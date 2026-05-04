package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"tastetheworld/internal/models"
)

// --- CONTROLADORES (HANDLERS) ---

func home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderizar(w, "./ui/html/index.html")
}

func ranking(w http.ResponseWriter, r *http.Request) {
	renderizar(w, "./ui/html/ranking.html")
}

func resenas(w http.ResponseWriter, r *http.Request) {
	renderizar(w, "./ui/html/resenas.html")
}

func registro(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderizar(w, "./ui/html/registro.html")
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		nuevoUsuario := models.Usuario{
			Nombre:   r.PostForm.Get("nombre"),
			Email:    r.PostForm.Get("email"),
			Password: r.PostForm.Get("password"),
		}

		guardarEnJSON(nuevoUsuario)
		// 3. Respondemos al usuario con una página de éxito (no solo texto raw)
		tmplSuccess, _ := template.New("success").Parse(`
		<!DOCTYPE html>
		<html lang="es">
		<head>
			<meta charset="utf-8">
			<title>¡Registro Completado! — TastetheWorld</title>
			<link rel="stylesheet" href="/static/css/main.css">
			<style>
				body { display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; background-color: #f9f9f9; font-family: sans-serif; }
				.confirmacion { background: white; padding: 3rem; border-radius: 8px; box-shadow: 0 4px 15px rgba(0,0,0,0.1); text-align: center; max-width: 450px; }
				h1 { color: #e67e22; margin-top: 0; }
				p { color: #555; margin-bottom: 2rem; line-height: 1.6; }
			</style>
		</head>
		<body>
			<div class="confirmacion">
				<h1>¡Gracias, {{.Nombre}}!</h1>
				<p>Te has registrado correctamente en TastetheWorld.<br>Tus datos se han guardado de forma permanente en el servidor.</p>
				<a href="/" class="boton boton--primario">Volver a la Página Principal</a>
			</div>
		</body>
		</html>
		`)

		// Ejecutamos la plantilla pasando el usuario para poner su nombre
		errTmpl := tmplSuccess.Execute(w, nuevoUsuario)
		if errTmpl != nil {
			log.Println(errTmpl)
			http.Error(w, "Error al mostrar confirmación", 500)
		}
		return // Importante poner el return para que no siga ejecutando código

		// --- BORRA LA LÍNEA ANTIGUA: fmt.Fprintf(w, "¡Gracias... ") ---
	}
}

// --- FUNCIONES AUXILIARES ---

func renderizar(w http.ResponseWriter, rutaHTML string) {
	ts, err := template.ParseFiles(rutaHTML)
	if err != nil {
		log.Printf("Error al cargar %s: %v", rutaHTML, err)
		http.Error(w, "Error interno del servidor", 500)
		return
	}
	ts.Execute(w, nil)
}

func guardarEnJSON(u models.Usuario) {
	archivo := "./data/usuarios.json"
	var usuarios []models.Usuario
	datos, _ := os.ReadFile(archivo)
	json.Unmarshal(datos, &usuarios)
	usuarios = append(usuarios, u)
	datosFinales, _ := json.MarshalIndent(usuarios, "", "  ")
	os.WriteFile(archivo, datosFinales, 0644)
}

// --- FUNCIÓN PRINCIPAL ---

func main() {
	mux := http.NewServeMux()

	// 1. Servidor de archivos estáticos (CSS, Imágenes)
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	// 2. Registro de rutas (Si no están aquí, Go dará error 404)
	mux.HandleFunc("/", home)
	mux.HandleFunc("/ranking", ranking)
	mux.HandleFunc("/resenas", resenas)
	mux.HandleFunc("/registro", registro)

	log.Println("Servidor funcionando en http://localhost:8000")
	err := http.ListenAndServe(":8000", mux)
	log.Fatal(err)
}
