// Package handlers contiene todos los controladores HTTP de la aplicación.
// Implementa la capa de presentación/control del patrón MVC:
//   - Model      → internal/models
//   - View       → ui/html (plantillas Go)
//   - Controller → internal/handlers (este paquete)
//
// Además expone la API REST bajo /api/v1/ para la PEC 3.
package handlers

import (
	"encoding/json"
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
type DatosBase struct {
	UsuarioNombre    string
	UsuarioIniciales string
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
// HANDLERS DE PÁGINAS HTML
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

// Admin sirve la página de administración de usuarios ("/admin/usuarios").
// Solo accesible si hay sesión activa; redirige a /login en caso contrario.
func (app *Aplicacion) Admin(w http.ResponseWriter, r *http.Request) {
	sesion := app.sesionActual(r)
	if sesion.UsuarioNombre == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	app.renderizar(w, "admin.html", sesion)
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
		app.renderizarRegistro(w, r, "Ese correo ya está registrado. ¿Quieres iniciar sesión?", datos)
		return
	}

	// Hashear con bcrypt (coste 12). Nunca texto plano.
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

	// Mensaje genérico: no revelar si el email existe o no (prevención de enumeración).
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

	// Cookie HttpOnly: no accesible desde JavaScript.
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
// Solo acepta POST para evitar logout accidental por GET (prefetch, etc.).
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
// API REST — /api/v1/usuarios
// ============================================================

// respuestaUsuario es la representación pública de un usuario en la API.
// Nunca incluye el hash de la contraseña.
type respuestaUsuario struct {
	Nombre string `json:"nombre"`
	Email  string `json:"email"`
}

// respuestaError es el formato estándar de error de la API.
type respuestaError struct {
	Mensaje string `json:"mensaje"`
}

// escribirJSON serializa v como JSON y lo envía con el código de estado dado.
func escribirJSON(w http.ResponseWriter, codigo int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// En este punto el status ya está enviado; solo registramos el error.
		_ = err
	}
}

// errorJSON envía un error JSON con el código y mensaje dados.
func errorJSON(w http.ResponseWriter, codigo int, mensaje string) {
	escribirJSON(w, codigo, respuestaError{Mensaje: mensaje})
}

// requireSesion comprueba que la petición tiene cookie de sesión válida.
// Devuelve true si está autenticado; false (y escribe 401) si no.
func (app *Aplicacion) requireSesion(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(nombreCookie)
	if err != nil {
		errorJSON(w, http.StatusUnauthorized, "Se requiere autenticación.")
		return false
	}
	if _, ok := app.SesionStore.Obtener(cookie.Value); !ok {
		errorJSON(w, http.StatusUnauthorized, "Sesión expirada o inválida.")
		return false
	}
	return true
}

// ApiUsuarios atiende GET /api/v1/usuarios y POST /api/v1/usuarios.
//
//	GET  → lista todos los usuarios (sin contraseña). 200 OK.
//	POST → crea un nuevo usuario. 201 Created / 400 / 409.
func (app *Aplicacion) ApiUsuarios(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.apiListarUsuarios(w, r)
	case http.MethodPost:
		app.apiCrearUsuario(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		errorJSON(w, http.StatusMethodNotAllowed, "Método no permitido.")
	}
}

// ApiUsuario atiende GET /api/v1/usuarios/{email}, PUT y DELETE.
//
//	GET    → busca un usuario por email. 200 / 404.
//	PUT    → actualiza el nombre. Requiere sesión. 200 / 400 / 401 / 404.
//	DELETE → elimina el usuario. Requiere sesión. 204 / 401 / 404.
func (app *Aplicacion) ApiUsuario(w http.ResponseWriter, r *http.Request) {
	// Extraer el email de la URL: /api/v1/usuarios/{email}
	email := strings.TrimPrefix(r.URL.Path, "/api/v1/usuarios/")
	if email == "" {
		errorJSON(w, http.StatusBadRequest, "Email no especificado en la URL.")
		return
	}

	switch r.Method {
	case http.MethodGet:
		app.apiBuscarUsuario(w, r, email)
	case http.MethodPut:
		if !app.requireSesion(w, r) {
			return
		}
		app.apiActualizarUsuario(w, r, email)
	case http.MethodDelete:
		if !app.requireSesion(w, r) {
			return
		}
		app.apiEliminarUsuario(w, r, email)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		errorJSON(w, http.StatusMethodNotAllowed, "Método no permitido.")
	}
}

// apiListarUsuarios devuelve el array completo de usuarios sin password_hash.
func (app *Aplicacion) apiListarUsuarios(w http.ResponseWriter, r *http.Request) {
	usuarios, err := app.UsuarioStore.LeerTodos()
	if err != nil {
		app.Log.Printf("api: listar usuarios: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	// Convertir a respuestaUsuario para no exponer el hash.
	respuesta := make([]respuestaUsuario, len(usuarios))
	for i, u := range usuarios {
		respuesta[i] = respuestaUsuario{Nombre: u.Nombre, Email: u.Email}
	}
	escribirJSON(w, http.StatusOK, respuesta)
}

// apiBuscarUsuario devuelve un único usuario por email.
func (app *Aplicacion) apiBuscarUsuario(w http.ResponseWriter, r *http.Request, email string) {
	usuario, err := app.UsuarioStore.BuscarPorEmail(email)
	if err != nil {
		app.Log.Printf("api: buscar usuario: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	if usuario == nil {
		errorJSON(w, http.StatusNotFound, "Usuario no encontrado.")
		return
	}
	escribirJSON(w, http.StatusOK, respuestaUsuario{Nombre: usuario.Nombre, Email: usuario.Email})
}

// apiCrearUsuario procesa el body JSON y crea un nuevo usuario.
func (app *Aplicacion) apiCrearUsuario(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Nombre   string `json:"nombre"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorJSON(w, http.StatusBadRequest, "JSON inválido.")
		return
	}

	body.Nombre = strings.TrimSpace(body.Nombre)
	body.Email = strings.TrimSpace(body.Email)

	if body.Nombre == "" || body.Email == "" || body.Password == "" {
		errorJSON(w, http.StatusBadRequest, "Los campos nombre, email y password son obligatorios.")
		return
	}
	if len(body.Password) < 8 {
		errorJSON(w, http.StatusBadRequest, "La contraseña debe tener al menos 8 caracteres.")
		return
	}

	existe, err := app.UsuarioStore.ExisteEmail(body.Email)
	if err != nil {
		app.Log.Printf("api: comprobar email: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	if existe {
		errorJSON(w, http.StatusConflict, "Ese correo electrónico ya está registrado.")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err != nil {
		app.Log.Printf("api: hashear contraseña: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}

	if err := app.UsuarioStore.Guardar(models.Usuario{
		Nombre:       body.Nombre,
		Email:        body.Email,
		PasswordHash: string(hash),
	}); err != nil {
		app.Log.Printf("api: guardar usuario: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}

	escribirJSON(w, http.StatusCreated, respuestaUsuario{Nombre: body.Nombre, Email: body.Email})
}

// apiActualizarUsuario actualiza el nombre del usuario indicado en la URL.
func (app *Aplicacion) apiActualizarUsuario(w http.ResponseWriter, r *http.Request, email string) {
	var body struct {
		Nombre string `json:"nombre"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorJSON(w, http.StatusBadRequest, "JSON inválido.")
		return
	}
	body.Nombre = strings.TrimSpace(body.Nombre)
	if body.Nombre == "" {
		errorJSON(w, http.StatusBadRequest, "El campo nombre es obligatorio.")
		return
	}

	actualizado, err := app.UsuarioStore.Actualizar(email, body.Nombre)
	if err != nil {
		app.Log.Printf("api: actualizar usuario: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	if !actualizado {
		errorJSON(w, http.StatusNotFound, "Usuario no encontrado.")
		return
	}

	escribirJSON(w, http.StatusOK, respuestaUsuario{Nombre: body.Nombre, Email: email})
}

// apiEliminarUsuario borra el usuario indicado en la URL.
func (app *Aplicacion) apiEliminarUsuario(w http.ResponseWriter, r *http.Request, email string) {
	eliminado, err := app.UsuarioStore.Eliminar(email)
	if err != nil {
		app.Log.Printf("api: eliminar usuario: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	if !eliminado {
		errorJSON(w, http.StatusNotFound, "Usuario no encontrado.")
		return
	}
	// 204 No Content: éxito sin cuerpo de respuesta.
	w.WriteHeader(http.StatusNoContent)
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
