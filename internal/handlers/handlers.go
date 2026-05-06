// Package handlers contiene todos los controladores HTTP de la aplicación.
// Implementa la capa de presentación/control del patrón MVC:
//   - Model      → internal/models
//   - View        → ui/html (plantillas Go)
//   - Controller → internal/handlers (este paquete)
package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"tastetheworld/internal/models"
	"tastetheworld/internal/storage"
)

// nombreCookie es el nombre de la cookie de sesión.
const nombreCookie = "ttw_sesion"

// Aplicacion agrupa las dependencias compartidas por todos los handlers.
type Aplicacion struct {
	Log          *log.Logger
	Plantillas   map[string]*template.Template
	UsuarioStore *storage.UsuarioStore
	SesionStore  *storage.SesionStore
}

// DatosBase contiene los campos que reciben TODAS las plantillas a través de base.html.
// Permite mostrar el icono de usuario con sus iniciales cuando hay sesión activa.
type DatosBase struct {
	UsuarioNombre    string // nombre completo del usuario (vacío si no autenticado)
	UsuarioIniciales string // ej: "AG" para "Ana García"
}

// sesionActual lee la cookie y devuelve los DatosBase con la sesión activa, si la hay.
func (app *Aplicacion) sesionActual(r *http.Request) DatosBase {
	cookie, err := r.Cookie(nombreCookie)
	if err != nil {
		return DatosBase{}
	}
	sesion, ok := app.SesionStore.Obtener(cookie.Value)
	if !ok {
		return DatosBase{}
	}
	return DatosBase{
		UsuarioNombre:    sesion.NombreUsuario,
		UsuarioIniciales: iniciales(sesion.NombreUsuario),
	}
}

// iniciales extrae hasta dos iniciales de un nombre completo.
// "Ana García" → "AG", "Ana" → "A", "" → "?"
func iniciales(nombre string) string {
	palabras := strings.Fields(nombre)
	if len(palabras) == 0 {
		return "?"
	}
	resultado := ""
	for i, p := range palabras {
		if i >= 2 {
			break
		}
		r, _ := utf8.DecodeRuneInString(p)
		if r != utf8.RuneError {
			resultado += strings.ToUpper(string(r))
		}
	}
	if resultado == "" {
		return "?"
	}
	return resultado
}

// ============================================================
// HANDLERS DE PÁGINAS
// ============================================================

// Home sirve la página de inicio ("/").
func (app *Aplicacion) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	app.renderizar(w, "index.html", app.sesionActual(r))
}

// Ranking sirve la página de ranking ("/ranking").
func (app *Aplicacion) Ranking(w http.ResponseWriter, r *http.Request) {
	app.renderizar(w, "ranking.html", app.sesionActual(r))
}

// Resenas sirve la página de reseñas con buscador ("/resenas").
func (app *Aplicacion) Resenas(w http.ResponseWriter, r *http.Request) {
	consulta := strings.TrimSpace(r.URL.Query().Get("buscar"))

	type DatosPaginaResenas struct {
		DatosBase
		Consulta  string
		HayFiltro bool
	}

	app.renderizar(w, "resenas.html", DatosPaginaResenas{
		DatosBase: app.sesionActual(r),
		Consulta:  consulta,
		HayFiltro: consulta != "",
	})
}

// ============================================================
// HANDLER DE REGISTRO
// ============================================================

// Registro gestiona GET y POST de "/registro".
func (app *Aplicacion) Registro(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.renderizarRegistro(w, r, "", nil)
	case http.MethodPost:
		app.procesarRegistro(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
	}
}

// DatosFormularioRegistro agrupa los datos que recibe la vista de registro.
type DatosFormularioRegistro struct {
	DatosBase
	MensajeError string
	MensajeExito string
	NombreValor  string
	EmailValor   string
}

func (app *Aplicacion) renderizarRegistro(w http.ResponseWriter, r *http.Request, mensajeError string, datos *DatosFormularioRegistro) {
	if datos == nil {
		datos = &DatosFormularioRegistro{}
	}
	datos.DatosBase = app.sesionActual(r)
	datos.MensajeError = mensajeError
	app.renderizar(w, "registro.html", datos)
}

func (app *Aplicacion) procesarRegistro(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		app.Log.Printf("handlers: parsear formulario registro: %v", err)
		http.Error(w, "No se pudo leer el formulario", http.StatusBadRequest)
		return
	}

	nombre := strings.TrimSpace(r.PostForm.Get("nombre"))
	email := strings.TrimSpace(r.PostForm.Get("email"))
	password := r.PostForm.Get("password")
	datos := &DatosFormularioRegistro{NombreValor: nombre, EmailValor: email}

	if nombre == "" || email == "" || password == "" {
		app.renderizarRegistro(w, r, "Todos los campos son obligatorios.", datos)
		return
	}
	if len(password) < 8 {
		app.renderizarRegistro(w, r, "La contraseña debe tener al menos 8 caracteres.", datos)
		return
	}

	existe, err := app.UsuarioStore.ExisteEmail(email)
	if err != nil {
		app.Log.Printf("handlers: comprobar email: %v", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}
	if existe {
		// Indicar al usuario que puede iniciar sesión con ese correo.
		app.renderizarRegistro(w, r, "Ese correo ya está registrado. ¿Quieres iniciar sesión?", datos)
		return
	}

	// Hashear con bcrypt (coste 12) según Tema 3. Nunca texto plano.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		app.Log.Printf("handlers: hashear contraseña: %v", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	if err := app.UsuarioStore.Guardar(models.Usuario{
		Nombre:       nombre,
		Email:        email,
		PasswordHash: string(hash),
	}); err != nil {
		app.Log.Printf("handlers: guardar usuario: %v", err)
		http.Error(w, "No se pudo guardar el usuario", http.StatusInternalServerError)
		return
	}

	datos.MensajeExito = nombre
	datos.NombreValor = ""
	datos.EmailValor = ""
	app.renderizar(w, "registro.html", datos)
}

// ============================================================
// HANDLER DE LOGIN
// ============================================================

// Login gestiona GET y POST de "/login".
//   - GET  → muestra el formulario de inicio de sesión.
//   - POST → verifica credenciales, crea sesión y redirige a inicio.
func (app *Aplicacion) Login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.renderizarLogin(w, r, "", "")
	case http.MethodPost:
		app.procesarLogin(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
	}
}

// DatosFormularioLogin agrupa los datos que recibe la vista de login.
type DatosFormularioLogin struct {
	DatosBase
	MensajeError string
	EmailValor   string
}

func (app *Aplicacion) renderizarLogin(w http.ResponseWriter, r *http.Request, mensajeError, emailValor string) {
	app.renderizar(w, "login.html", DatosFormularioLogin{
		DatosBase:    app.sesionActual(r),
		MensajeError: mensajeError,
		EmailValor:   emailValor,
	})
}

func (app *Aplicacion) procesarLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		app.Log.Printf("handlers: parsear formulario login: %v", err)
		http.Error(w, "No se pudo leer el formulario", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(r.PostForm.Get("email"))
	password := r.PostForm.Get("password")

	if email == "" || password == "" {
		app.renderizarLogin(w, r, "Introduce tu correo y contraseña.", email)
		return
	}

	usuario, err := app.UsuarioStore.BuscarPorEmail(email)
	if err != nil {
		app.Log.Printf("handlers: buscar usuario: %v", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	// Mensaje genérico: nunca revelar si el email existe o no (Tema 3).
	if usuario == nil || bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(password)) != nil {
		app.renderizarLogin(w, r, "Credenciales incorrectas.", email)
		return
	}

	// Crear sesión con token aleatorio de 256 bits.
	token, err := app.SesionStore.Crear(usuario.Nombre, usuario.Email)
	if err != nil {
		app.Log.Printf("handlers: crear sesión: %v", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	// Establecer la cookie de sesión (HttpOnly: no accesible desde JS).
	http.SetCookie(w, &http.Cookie{
		Name:     nombreCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   86400, // 24 horas
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// POST-Redirect-GET: evita reenvío del formulario al recargar.
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ============================================================
// HANDLER DE LOGOUT
// ============================================================

// Logout elimina la sesión y la cookie, luego redirige a inicio.
// Solo acepta POST para evitar logout accidental por GET (p.ej. prefetch).
func (app *Aplicacion) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	if cookie, err := r.Cookie(nombreCookie); err == nil {
		app.SesionStore.Eliminar(cookie.Value)
	}

	// MaxAge=-1 hace que el navegador borre la cookie inmediatamente.
	http.SetCookie(w, &http.Cookie{
		Name:     nombreCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ============================================================
// HELPER DE RENDERIZADO
// ============================================================

// renderizar ejecuta la plantilla cacheada identificada por nombre.
func (app *Aplicacion) renderizar(w http.ResponseWriter, nombre string, datos interface{}) {
	ts, ok := app.Plantillas[nombre]
	if !ok {
		app.Log.Printf("handlers: plantilla no encontrada: %s", nombre)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}
	if err := ts.ExecuteTemplate(w, "base", datos); err != nil {
		app.Log.Printf("handlers: ejecutar plantilla %s: %v", nombre, err)
	}
}
