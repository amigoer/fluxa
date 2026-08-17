// Package session implements the server-side session described in
// DESIGN.md 7.1: the client only ever holds an opaque random token in a
// cookie, and the server stores a hash of it in Postgres. Revoking a
// session (an admin forcing someone out, or a plain logout) takes effect
// on the very next request, which is the whole reason this was chosen
// over a stateless JWT.
package session

import (
	"time"

	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/service"
)

// sessionTTL is how long a session stays valid without the member
// logging in again. There is no sliding renewal in v1: it is simpler,
// and a week is generous enough for daily internal-tool use.
const sessionTTL = 7 * 24 * time.Hour

// Manager is the package's assembly point: it holds the collaborators
// the three halves of a session -- issuing one, ending one, and checking
// one on each request -- share. Each of those lives in its own file next
// to this one.
type Manager struct {
	repo         repo.Repo
	users        service.Service
	cookieName   string
	cookieSecure bool
}

func NewManager(repo repo.Repo, users service.Service, cookieName string, cookieSecure bool) *Manager {
	return &Manager{repo: repo, users: users, cookieName: cookieName, cookieSecure: cookieSecure}
}
