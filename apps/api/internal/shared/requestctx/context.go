package requestctx

import "context"

type key string

const (
	companyKey key = "company_id"
	userKey    key = "user_id"
	roleKey    key = "role"
)

type Identity struct {
	CompanyID string
	UserID    string
	Role      string
}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	ctx = context.WithValue(ctx, companyKey, identity.CompanyID)
	ctx = context.WithValue(ctx, userKey, identity.UserID)
	return context.WithValue(ctx, roleKey, identity.Role)
}

func FromContext(ctx context.Context) (Identity, bool) {
	companyID, companyOK := ctx.Value(companyKey).(string)
	userID, userOK := ctx.Value(userKey).(string)
	role, roleOK := ctx.Value(roleKey).(string)
	return Identity{CompanyID: companyID, UserID: userID, Role: role}, companyOK && userOK && roleOK
}
