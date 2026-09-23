package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const schema = `
CREATE TABLE IF NOT EXISTS admin_panel_users (
 username text PRIMARY KEY,
 password_hash text NOT NULL,
 must_change_password boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS admin_panel_sessions (
 token_hash text PRIMARY KEY,
 username text NOT NULL REFERENCES admin_panel_users(username) ON DELETE CASCADE,
 csrf_token text NOT NULL,
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS admin_panel_session_expiry ON admin_panel_sessions(expires_at);
CREATE TABLE IF NOT EXISTS admin_panel_audit (
 id bigserial PRIMARY KEY,
 actor text NOT NULL,
 action text NOT NULL,
 vendor_ids text[] NOT NULL DEFAULT '{}',
 before_revision text NOT NULL DEFAULT '',
 after_revision text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);`

type Identity struct {
	Username   string    `json:"username"`
	MustChange bool      `json:"must_change_password"`
	CSRF       string    `json:"csrf_token"`
	Expires    time.Time `json:"expires_at"`
}

var errCredentials = errors.New("invalid credentials")

func token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func digest(s string) string      { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func validPassword(s string) bool { return utf8.RuneCountInString(s) >= 12 && len(s) <= 72 }

// Initialize creates the first admin only. Repeated setup never resets a password.
func Initialize(ctx context.Context, pool *pgxpool.Pool, initialPassword string) (string, bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(79479003)"); err != nil {
		return "", false, err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return "", false, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM admin_panel_users WHERE username='admin')").Scan(&exists); err != nil {
		return "", false, err
	}
	if exists {
		return "", false, tx.Commit(ctx)
	}
	if initialPassword == "" {
		initialPassword = "Adm-" + token()[:24]
	}
	if !validPassword(initialPassword) {
		return "", false, fmt.Errorf("initial password must be 12+ characters and at most 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(initialPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", false, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO admin_panel_users(username,password_hash) VALUES('admin',$1)", string(hash)); err != nil {
		return "", false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", false, err
	}
	return initialPassword, true, nil
}

func (a *App) login(ctx context.Context, username, password string) (Identity, string, error) {
	var user Identity
	var hash string
	err := a.pool.QueryRow(ctx, "SELECT username,password_hash,must_change_password FROM admin_panel_users WHERE username=$1", username).Scan(&user.Username, &hash, &user.MustChange)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(password))
		return user, "", errCredentials
	}
	if err != nil {
		return user, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return user, "", errCredentials
	}
	raw := token()
	user.CSRF = token()
	user.Expires = time.Now().Add(8 * time.Hour)
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return user, "", err
	}
	defer tx.Rollback(ctx)
	// Synchronize against password rotation: a login cannot validate the old hash
	// and create a session after the password-change transaction has committed.
	var current string
	if err = tx.QueryRow(ctx, "SELECT password_hash FROM admin_panel_users WHERE username=$1 FOR UPDATE", username).Scan(&current); err != nil {
		return user, "", err
	}
	if current != hash {
		return user, "", errCredentials
	}
	if _, err = tx.Exec(ctx, "DELETE FROM admin_panel_sessions WHERE expires_at <= now()"); err != nil {
		return user, "", err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO admin_panel_sessions(token_hash,username,csrf_token,expires_at) VALUES($1,$2,$3,$4)", digest(raw), username, user.CSRF, user.Expires); err != nil {
		return user, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return user, "", err
	}
	return user, raw, nil
}

func (a *App) identity(ctx context.Context, raw string) (Identity, error) {
	var user Identity
	if len(raw) != 64 {
		return user, errCredentials
	}
	err := a.pool.QueryRow(ctx, `SELECT u.username,u.must_change_password,s.csrf_token,s.expires_at FROM admin_panel_sessions s JOIN admin_panel_users u ON u.username=s.username WHERE s.token_hash=$1 AND s.expires_at>now()`, digest(raw)).Scan(&user.Username, &user.MustChange, &user.CSRF, &user.Expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return user, errCredentials
	}
	return user, err
}

func (a *App) changePassword(ctx context.Context, user Identity, old, newPassword string) error {
	if !validPassword(newPassword) || newPassword == old {
		return fmt.Errorf("password must differ and contain 12+ characters (maximum 72 bytes)")
	}
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var hash string
	if err = tx.QueryRow(ctx, "SELECT password_hash FROM admin_panel_users WHERE username=$1 FOR UPDATE", user.Username).Scan(&hash); err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(old)) != nil {
		return errCredentials
	}
	updated, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE admin_panel_users SET password_hash=$1,must_change_password=false WHERE username=$2", string(updated), user.Username); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM admin_panel_sessions WHERE username=$1", user.Username); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO admin_panel_audit(actor,action) VALUES($1,'password_changed')", user.Username); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
