package settingsdb

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/mosimosi228/ovpn-dash/internal/settingsdb/sqlitedb"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
)

// User is a dashboard account.
type User struct {
	ID             int64
	Email          string
	Name           string
	PassHash       string
	Role           string
	ClientName     string
	TelegramChatID string
	TelegramBind   string
	Theme          string
	MapStyle       string
	Disabled       bool
	CreatedAt      string
	UpdatedAt      string
}

func fromRow(row sqlitedb.User) User {
	return User{
		ID:             row.ID,
		Email:          row.Email.String,
		Name:           row.Name,
		PassHash:       row.PassHash,
		Role:           row.Role,
		ClientName:     row.ClientName.String,
		TelegramChatID: row.TelegramChatID.String,
		TelegramBind:   row.TelegramBind.String,
		Theme:          row.Theme,
		MapStyle:       row.MapStyle,
		Disabled:       row.Disabled != 0,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func ns(s string) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func (u User) insertParams() sqlitedb.InsertUserParams {
	now := time.Now().UTC().Format(time.RFC3339)
	theme := u.Theme
	if theme == "" {
		theme = setup.ThemeLight
	}
	mapStyle := u.MapStyle
	if mapStyle == "" {
		mapStyle = setup.MapAuto
	}
	created := u.CreatedAt
	if created == "" {
		created = now
	}
	return sqlitedb.InsertUserParams{
		Email:          ns(strings.ToLower(strings.TrimSpace(u.Email))),
		Name:           strings.TrimSpace(u.Name),
		PassHash:       u.PassHash,
		Role:           u.Role,
		ClientName:     ns(u.ClientName),
		TelegramChatID: ns(u.TelegramChatID),
		TelegramBind:   ns(u.TelegramBind),
		Theme:          theme,
		MapStyle:       mapStyle,
		Disabled:       boolToInt(u.Disabled),
		CreatedAt:      created,
		UpdatedAt:      now,
	}
}

func (u User) updateParams() sqlitedb.UpdateUserParams {
	now := time.Now().UTC().Format(time.RFC3339)
	theme := u.Theme
	if theme == "" {
		theme = setup.ThemeLight
	}
	mapStyle := u.MapStyle
	if mapStyle == "" {
		mapStyle = setup.MapAuto
	}
	return sqlitedb.UpdateUserParams{
		Email:          ns(strings.ToLower(strings.TrimSpace(u.Email))),
		Name:           strings.TrimSpace(u.Name),
		PassHash:       u.PassHash,
		Role:           u.Role,
		ClientName:     ns(u.ClientName),
		TelegramChatID: ns(u.TelegramChatID),
		TelegramBind:   ns(u.TelegramBind),
		Theme:          theme,
		MapStyle:       mapStyle,
		Disabled:       boolToInt(u.Disabled),
		UpdatedAt:      now,
		ID:             u.ID,
	}
}

func boolToInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func (d *DB) CountUsers(ctx context.Context) (int64, error) {
	return d.Q.CountUsers(ctx)
}

func (d *DB) GetUserByID(ctx context.Context, id int64) (User, error) {
	row, err := d.Q.GetUserByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	return fromRow(row), nil
}

func (d *DB) GetUserByEmail(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, sql.ErrNoRows
	}
	row, err := d.Q.GetUserByEmail(ctx, sql.NullString{String: email, Valid: true})
	if err != nil {
		return User{}, err
	}
	return fromRow(row), nil
}

// FindLogin resolves an email, client name (CN), display name, or v1 username.
func (d *DB) FindLogin(ctx context.Context, ident string) (User, error) {
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return User{}, sql.ErrNoRows
	}
	if u, err := d.GetUserByEmail(ctx, ident); err == nil {
		return u, nil
	}
	users, err := d.ListUsers(ctx)
	if err != nil {
		return User{}, err
	}
	if u, ok := uniqueUser(users, func(u User) bool {
		return u.ClientName != "" && strings.EqualFold(u.ClientName, ident)
	}); ok {
		return u, nil
	}
	if u, ok := uniqueUser(users, func(u User) bool {
		return strings.EqualFold(u.Name, ident)
	}); ok {
		return u, nil
	}
	if !strings.Contains(ident, "@") {
		if u, ok := uniqueUser(users, func(u User) bool {
			local, _, _ := strings.Cut(u.Email, "@")
			return strings.EqualFold(local, ident)
		}); ok {
			return u, nil
		}
		legacy, _ := d.GetMeta(ctx, setup.KeyAdminUser)
		if strings.EqualFold(strings.TrimSpace(legacy), ident) {
			if u, ok := uniqueUser(users, func(u User) bool {
				return u.Role == setup.RoleRoot
			}); ok {
				return u, nil
			}
		}
	}
	return User{}, sql.ErrNoRows
}

func uniqueUser(users []User, match func(User) bool) (User, bool) {
	var found User
	n := 0
	for _, u := range users {
		if match(u) {
			found = u
			n++
		}
	}
	if n == 1 {
		return found, true
	}
	return User{}, false
}

func (d *DB) GetUserByClientName(ctx context.Context, name string) (User, error) {
	row, err := d.Q.GetUserByClientName(ctx, ns(name))
	if err != nil {
		return User{}, err
	}
	return fromRow(row), nil
}

func (d *DB) GetUserByTelegramChatID(ctx context.Context, chatID string) (User, error) {
	row, err := d.Q.GetUserByTelegramChatID(ctx, ns(chatID))
	if err != nil {
		return User{}, err
	}
	return fromRow(row), nil
}

func (d *DB) GetUserByTelegramBind(ctx context.Context, code string) (User, error) {
	row, err := d.Q.GetUserByTelegramBind(ctx, ns(code))
	if err != nil {
		return User{}, err
	}
	return fromRow(row), nil
}

func (d *DB) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := d.Q.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromRow(row))
	}
	return out, nil
}

func (d *DB) InsertUser(ctx context.Context, u User) (User, error) {
	res, err := d.Q.InsertUser(ctx, u.insertParams())
	if err != nil {
		return User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return d.GetUserByID(ctx, id)
}

func (d *DB) UpdateUser(ctx context.Context, u User) error {
	return d.Q.UpdateUser(ctx, u.updateParams())
}

func (d *DB) DeleteUser(ctx context.Context, id int64) error {
	return d.Q.DeleteUser(ctx, id)
}

func (u User) IsStaff() bool {
	return u.Role == setup.RoleRoot || u.Role == setup.RoleAdmin
}

func (u User) Public() map[string]any {
	return map[string]any{
		"id":             u.ID,
		"email":          u.Email,
		"name":           u.Name,
		"role":           u.Role,
		"client_name":    u.ClientName,
		"telegram_bound": u.TelegramChatID != "",
		"theme":          u.Theme,
		"map_style":      u.MapStyle,
		"disabled":       u.Disabled,
		"created_at":     u.CreatedAt,
		"updated_at":     u.UpdatedAt,
	}
}
