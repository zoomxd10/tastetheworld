# TasteTheWorld 🌍🍜

A small food-review web application written in **Go** using only the standard library (plus `bcrypt`). Users can register, log in, browse a ranking of world cuisines and read/write reviews. It also includes an admin page and a REST API to manage users.

Built by **Antonio Galván Gerona** and **Inés Téllez** ([@inestellez](https://github.com/inestellez)) as the course project for *Redes y Sistemas Web* (Web Networks & Systems), 2nd year of Computer Engineering + Robotics. The commit history follows the three project milestones.

## Features

- Server-side rendered pages with `html/template` and a shared base layout (templates parsed once at startup and cached)
- User registration and login with passwords hashed using **bcrypt** (never stored in plain text)
- Session handling with random 256-bit tokens stored in an `HttpOnly`, `SameSite=Lax` cookie, expiring after 24 h
- Admin page (`/admin/usuarios`) to manage users
- JSON **REST API** with full CRUD for users, protected by session
- MVC-style layout: `cmd/` (entry point), `internal/handlers` (controllers), `internal/models`, `internal/storage` (JSON file + in-memory session store), `ui/` (views and static assets)
- Styles written in **SCSS** (7-1-like structure: abstracts, base, components, layout, pages) compiled to `main.css`

## REST API

| Method | Route | Description |
|---|---|---|
| `GET` | `/api/v1/usuarios` | List users |
| `POST` | `/api/v1/usuarios` | Create a user |
| `GET` | `/api/v1/usuarios/{email}` | Get one user |
| `PUT` | `/api/v1/usuarios/{email}` | Update a user |
| `DELETE` | `/api/v1/usuarios/{email}` | Delete a user |

## Running it

Requires Go 1.22+.

```bash
go run ./cmd/web
```

Then open <http://localhost:8000>. Users are stored in `data/usuarios.json`.

To recompile the styles after editing the SCSS (requires [Sass](https://sass-lang.com/)):

```bash
sass ui/static/scss/main.scss ui/static/css/main.css
```

## Project structure

```
cmd/web/            entry point, routes and template cache
internal/handlers/  HTTP handlers (pages + REST API)
internal/models/    data structures
internal/storage/   user store (JSON file) and session store
ui/html/            Go templates
ui/static/          SCSS sources, compiled CSS and images
```

---

## 🇪🇸 En español

**TasteTheWorld** es una aplicación web de reseñas gastronómicas hecha en **Go** con la librería estándar (y `bcrypt`). Permite registrarse, iniciar sesión, ver un ranking de cocinas del mundo y leer o escribir reseñas. Además incluye un panel de administración y una API REST para gestionar usuarios.

La hicimos **Antonio Galván Gerona** e **Inés Téllez** ([@inestellez](https://github.com/inestellez)) como proyecto de la asignatura *Redes y Sistemas Web* (2º de Ingeniería Informática + Robótica). El historial de commits sigue las tres entregas del proyecto.

**Lo más destacable:** contraseñas cifradas con bcrypt, sesiones con token aleatorio de 256 bits en una cookie `HttpOnly`, API REST con CRUD completo, plantillas cacheadas al arrancar, estructura tipo MVC y estilos en SCSS.

**Cómo ejecutarlo:** con Go 1.22 o superior, lanza `go run ./cmd/web` y abre <http://localhost:8000>.
