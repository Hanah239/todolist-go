# Task: Add change-password and change-email to todolist-go

You are working in an existing Go backend (`todolist-go`): plain `net/http`, SQLite via `modernc.org/sqlite` (package-level `db`), JWT auth via `golang-jwt/jwt/v5`, bcrypt via `golang.org/x/crypto`. Files: `main.go`, `auth.go`, `login.go`, `profile.go`, `tags.go`, `todos.go`, plus `README.md`, `api-doc/`, `postman/`.

Implement two features in two phases. **Finish and verify Phase 1 before starting Phase 2.**

---

## 0. Ground rules (read first)

1. **Before writing any code, read every `.go` file.** Learn and match the existing conventions: how the user ID is read from the JWT middleware, how JSON is decoded, how errors and success responses are written, how routes are registered, where tables are created, how register validates passwords/emails and which bcrypt cost it uses. Reuse existing helpers; do not invent parallel ones.
2. **No new third-party dependencies.** Use only the standard library plus what is already in `go.mod`.
3. **Do not change existing behavior** of register, login, todos, tags, profile, or role logic.
4. **Do not commit, push, or touch git history.** Leave changes in the working tree.
5. Never log passwords, hashes, or raw tokens (the one exception is the dev email link in Phase 2, specified below).
6. Keep the code in the style of the repo (error handling, naming, comments). Small, readable functions. Put new handlers in a new file `account.go`.
7. If something in this spec conflicts with the existing code, **stop and report the conflict** instead of guessing.

---

## Phase 1: Change password

### Endpoint
`PUT /change-password`, protected by the existing JWT middleware.

### Request
```json
{ "current_password": "string", "new_password": "string" }
```

### Behavior (in this order)
1. Get the authenticated user ID using the existing middleware mechanism.
2. Decode the body. Malformed JSON or missing/empty fields → `400`.
3. Load the user's stored password hash from `users` by ID. User not found → `401`.
4. `bcrypt.CompareHashAndPassword(hash, current_password)`. Mismatch → `401`, message "current password is incorrect". **Do not modify anything.**
5. Validate `new_password`: apply the same minimum-length/strength rule register uses (if register has none, require at least 8 characters). Failure → `400`.
6. Reject if `new_password` equals the current password (compare the new plaintext against the existing hash with bcrypt). → `400`, "new password must be different".
7. Hash `new_password` with the same bcrypt cost as register.
8. `UPDATE users SET <password column> = ? WHERE id = ?`.
9. Return `200` with `{"message":"password changed"}` (match the repo's response style).

### Known limitation (document it, do not fix)
JWTs issued before the change remain valid until they expire, because tokens are stateless. Note this in the README.

### Phase 1 acceptance
- Wrong current password → 401, password unchanged (login with old password still works).
- Short new password → 400. Same-as-old → 400. Missing token → 401 from middleware.
- Correct request → 200; login with the **old** password now fails; login with the **new** password succeeds.

---

## Phase 2: Change email with verification

### Design
The user's real email **must not change** until the link is confirmed. The pending change lives in its own table. The activation link is single-use and expires.

### Schema
Create next to the existing `CREATE TABLE IF NOT EXISTS` statements (so it runs on startup):

```sql
CREATE TABLE IF NOT EXISTS email_changes (
  user_id    INTEGER PRIMARY KEY,
  new_email  TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  expires_at DATETIME NOT NULL
);
```
`user_id` as PK means one pending change per user. A new request replaces the previous one (`INSERT OR REPLACE`), which invalidates the older link.

### 2a. `POST /change-email` (JWT protected)

Request:
```json
{ "current_password": "string", "new_email": "string" }
```

Behavior (in this order):
1. Get the user ID from the middleware. Decode body; empty fields → `400`.
2. Normalize `new_email` exactly the way register does (trim, lowercase if register lowercases). Validate format with the same check register uses (if none, use `net/mail.ParseAddress` and require the parsed address to equal the input). Invalid → `400`.
3. Load the user's hash and verify `current_password` with bcrypt. Mismatch → `401`. **Nothing is written.**
4. If `new_email` equals the user's current email → `400`.
5. If another user already has `new_email` → `409`.
6. Generate a token: 32 bytes from `crypto/rand`, hex-encoded (64 chars).
7. Store `sha256(token)` as hex in `token_hash` (**never store the raw token**). `expires_at` = now + 1 hour (UTC).
8. `INSERT OR REPLACE INTO email_changes ...`.
9. Call `sendEmail(to, subject, body)` (see below) with a link: `{BASE_URL}/verify-email?token={rawToken}`.
10. Return `200` `{"message":"verification link sent to the new email"}`. Do **not** return the token in the response.

### 2b. `GET /verify-email?token=...` (public, no JWT)

The token is the proof of ownership, so no JWT is required (the user may open the link on another device).

Behavior:
1. Read `token` query param. Missing → `400`.
2. Compute `sha256(token)` hex; look up the row by `token_hash`.
3. No row → `400` "invalid or expired link". Use the **same message** for not-found and expired.
4. If `expires_at` has passed (compare in Go using UTC) → delete the row, then `400` "invalid or expired link".
5. Inside a **single DB transaction**:
   - Re-check that `new_email` is not now used by another user. If it is → delete the pending row, commit, return `409`.
   - `UPDATE users SET email = new_email WHERE id = user_id`.
   - `DELETE FROM email_changes WHERE user_id = ?` (this is what makes the link single-use).
   - Commit.
6. Return `200` `{"message":"email verified"}`.

### 2c. Email sending stub

Create `sendEmail(to, subject, body string) error` in one place. For now it **prints to the terminal** (clearly marked, e.g. `[DEV EMAIL] to=... link=...`) instead of using SMTP. Keep it the only place that would need to change to add real SMTP later.

`BASE_URL` comes from an env var, defaulting to `http://localhost:<the port the server already uses>`.

### Interactions to check and REPORT (do not silently change)
- Does any code derive **admin/role** from the email (e.g. an `ADMIN_EMAIL` env var evaluated at login or in middleware)? If so, explain how changing email would interact with it.
- Does the **JWT carry an email claim** that would go stale after a change? Report what you find.
- Does anything else in the code assume email never changes?

### Phase 2 acceptance
- Wrong current password → 401, no `email_changes` row created.
- Email already taken (request time) → 409.
- Valid request → 200, link printed in terminal, `users.email` **unchanged** at this point.
- Open the link → 200; `users.email` is the new one; the `email_changes` row is gone.
- Open the **same link again** → 400.
- Link with an expired `expires_at` (set manually in the DB for the test) → 400, row removed.
- Two requests in a row: only the **second** link works; the first returns 400.
- Email taken between request and verify → 409 on verify.

---

## Status code summary

| Case | Code |
|---|---|
| Bad/missing body or fields, validation failure | 400 |
| Missing/invalid JWT, wrong current password | 401 |
| Email already in use | 409 |
| Invalid, expired, or used verification link | 400 |
| Success | 200 |

---

## Docs and tooling updates

1. **README.md:** add the three endpoints, the dev-email behavior, `BASE_URL`, and the stateless-JWT limitation.
2. **api-doc/:** add the three endpoints in the same format as the existing ones.
3. **postman/:** add requests for the new endpoints to the exported collection (JSON), using the existing `{{baseUrl}}`/`{{token}}` variables if present.

---

## Verification you must perform

Run the app, then exercise every acceptance case above with `curl` (or a shell script saved as `scripts/test-account.sh`). Show the commands and the real responses. Run `go vet ./...` and `go build ./...`; both must pass. If a case can't be tested, say why.

## Final report (required)

1. List of files created/changed.
2. The route registrations you added.
3. Results of each acceptance test.
4. Answers to the "Interactions to check and REPORT" section.
5. Any deviations from this spec, and any open questions.
6. Known limitations (stateless JWT after password change, no rate limiting on `/change-email`, dev-only email).
