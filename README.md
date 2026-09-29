# todolist-go

A backend-only REST API in Go with SQLite: user registration, login,
role-based authorization, a personal to-do list, tags, and profiles.

## Why I built this

I built this to learn how authentication, authorization, and a to-do list work
under the hood, using Go and SQLite. Coming from MySQL and Python at my HND, I
wanted to see how a backend handles passwords, sessions, and per-user data
without a framework doing it for me.

## Requirements

- Go 1.22 or newer (uses the built-in method routing)
- No separate database install: SQLite is embedded through a pure-Go driver

## Run it

```bash
go mod tidy
ADMIN_EMAIL=admin@test.com go run .
```

The server listens on `:8080`. The account registered with `ADMIN_EMAIL`
becomes an admin. That variable is the only configuration.

## Endpoints

| Method | Path | Access |
|---|---|---|
| POST | /register | public |
| POST | /login | public, returns a token |
| POST | /logout | logged in |
| GET | /me | logged in |
| GET, POST | /todos | logged in, your own to-dos |
| PUT, DELETE | /todos/{id} | logged in, owner only |
| GET | /profile | logged in, your own profile |
| PUT | /profile | logged in, your own profile |
| GET | /tags | logged in, lists all tags |
| POST | /todos/{id}/tags | logged in, owner only |
| DELETE | /todos/{id}/tags/{tagID} | logged in, owner only |
| GET | /admin/todos | admin only |
| GET | /admin/ping | admin only |

Send the token as `Authorization: Bearer <token>`.

## Example

```bash
curl -X POST localhost:8080/register -d '{"email":"a@test.com","password":"password123"}'
curl -X POST localhost:8080/login -d '{"email":"a@test.com","password":"password123"}'
# -> {"token":"..."}

curl -X POST localhost:8080/todos -H "Authorization: Bearer <token>" -d '{"title":"learn Go"}'
curl localhost:8080/todos -H "Authorization: Bearer <token>"

curl -X POST localhost:8080/todos/1/tags -H "Authorization: Bearer <token>" -d '{"name":"urgent","color":"#ff0000"}'
```

## Project structure

- `main.go`: database setup, registration, routes
- `login.go`: login and token creation
- `auth.go`: token check, roles, logout
- `todos.go`: to-do CRUD, tag lookups, and the admin listing
- `tags.go`: tag creation and attaching/removing tags on a to-do
- `profile.go`: view and update the user's profile

## Design notes

- Passwords are hashed with bcrypt, never stored in plain text.
- Sessions are random tokens stored in the database, valid for 24 hours.
  Logging out deletes the token.
- Every to-do query filters on the owner's id, so users can't read or
  change each other's data. Someone else's to-do returns 404, the same
  as a missing one.
- Login returns the same error for a wrong email and a wrong password.
- **Profile** is a one-to-one relationship: each user has at most one
  profile row (`user_id` is the primary key of the `profiles` table).
- **Tags** are a many-to-many relationship: a to-do can have many tags,
  and a tag can be reused across many to-dos, linked through a
  `todo_tags` junction table. Tag names are global and shared across
  all users; which to-do has which tag stays scoped to its owner.

## Known limitations

- No rate limiting on login and no HTTPS.
- Expired sessions are never cleaned up.
- The admin is chosen by an environment variable at registration, a
  shortcut for learning.
- Registration reveals whether an email is already taken.
- No database migrations: changing a table means deleting `app.db`.
- Session tokens are stored in plain text in the database.
- Tags are visible globally by name, even though usage stays private.

## Roadmap here on:

- `priority` and `due_date` on to-dos
- Timestamps (`created_at`, `updated_at`) and soft deletes (`deleted_at`)
- Filtering and searching to-dos
- `/change-password` and `/change-email`(token validation required)
- Dockerize and provide a Dockerfile
