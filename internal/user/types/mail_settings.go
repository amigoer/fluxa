package types

// MailSettings is the wording a deployment may set on the mails this
// system sends. The markup, colours and mark are not here on purpose --
// see the migration and notify.Brand for why.
type MailSettings struct {
	OrgName     string
	SignOff     string
	ContactLine string
}
