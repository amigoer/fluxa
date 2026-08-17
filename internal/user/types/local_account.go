package types

import "time"

type LocalAccount struct {
	ID        string
	MemberID  string
	Phone     *string
	Email     *string
	CreatedAt time.Time
}
