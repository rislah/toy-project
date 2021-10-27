package queries

import (
	"context"
	"strconv"
	"strings"

	"github.com/graph-gophers/graphql-go"
	app "github.com/rislah/fakes/internal"
	"github.com/rislah/fakes/loaders"
)

type RoleResolver struct {
	role *app.Role
	data *app.Data
}

type QueryRoleArgs struct {
	ID   *graphql.ID
	Name *app.RoleType
}

func NewRoleResolver(r *app.Role, data *app.Data) *RoleResolver {
	return &RoleResolver{role: r, data: data}
}

func (r *QueryResolver) Roles(ctx context.Context) ([]*RoleResolver, error) {
	return NewRoleListResolver(ctx, r.Data)
}

func (q *QueryResolver) Role(ctx context.Context, args QueryRoleArgs) (*RoleResolver, error) {
	if args.Name != nil {
		return NewRoleResolverByName(ctx, q.Data, strings.ToLower(args.Name.String()))
	}

	if args.ID != nil {
		return NewRoleResolverByID(ctx, q.Data, string(*args.ID))
	}

	return nil, nil
}

func NewRoleListResolver(ctx context.Context, data *app.Data) ([]*RoleResolver, error) {
	roles, err := data.RoleDB.GetRoles(ctx)
	if err != nil {
		return nil, err
	}

	loaders.PrimeRoles(ctx, roles)

	roleResolvers := make([]*RoleResolver, 0, len(roles))
	for _, role := range roles {
		roleResolvers = append(roleResolvers, NewRoleResolver(role, data))
	}

	return roleResolvers, nil
}

func NewRoleResolverByName(ctx context.Context, data *app.Data, name string) (*RoleResolver, error) {
	role, err := loaders.LoadRoleByName(ctx, name)
	if err != nil {
		return nil, err
	}

	if role == nil {
		return nil, nil
	}

	return NewRoleResolver(role, data), nil
}

func NewRoleResolverByID(ctx context.Context, data *app.Data, id string) (*RoleResolver, error) {
	role, err := loaders.LoadRoleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if role == nil {
		return nil, nil
	}

	return NewRoleResolver(role, data), nil
}

func NewRoleResolverByUserID(ctx context.Context, data *app.Data, userID string) (*RoleResolver, error) {
	role, err := loaders.LoadRoleByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if role == nil {
		return nil, nil
	}

	return NewRoleResolver(role, data), nil
}

func (r *RoleResolver) ID() graphql.ID {
	return graphql.ID(strconv.Itoa(r.role.ID))
}

func (r *RoleResolver) Name() string {
	return strings.ToUpper(r.role.Name.String())
}

func (r *RoleResolver) Users(ctx context.Context) (*[]*UserResolver, error) {
	return NewUsersByRoleIDResolver(ctx, r.data, r.role.ID)
}
