package repo

import (
	"context"
	"fmt"
	"log/slog"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samber/do"

	"github.com/Y-vQv-Y/DevLoom/backend/biz/agentresource"
	"github.com/Y-vQv-Y/DevLoom/backend/config"
	"github.com/Y-vQv-Y/DevLoom/backend/consts"
	"github.com/Y-vQv-Y/DevLoom/backend/db"
	"github.com/Y-vQv-Y/DevLoom/backend/db/image"
	"github.com/Y-vQv-Y/DevLoom/backend/db/teamgroup"
	"github.com/Y-vQv-Y/DevLoom/backend/db/teamgrouphost"
	"github.com/Y-vQv-Y/DevLoom/backend/db/teamgroupimage"
	"github.com/Y-vQv-Y/DevLoom/backend/db/teamgroupmember"
	"github.com/Y-vQv-Y/DevLoom/backend/db/teamimage"
	"github.com/Y-vQv-Y/DevLoom/backend/db/teammember"
	"github.com/Y-vQv-Y/DevLoom/backend/db/user"
	"github.com/Y-vQv-Y/DevLoom/backend/domain"
	"github.com/Y-vQv-Y/DevLoom/backend/errcode"
	"github.com/Y-vQv-Y/DevLoom/backend/pkg/crypto"
	"github.com/Y-vQv-Y/DevLoom/backend/pkg/entx"
)

// TeamGroupUserRepo 团队分组成员数据访问层
type TeamGroupUserRepo struct {
	db     *db.Client
	redis  *redis.Client
	config *config.Config
	logger *slog.Logger
}

const (
	defaultTeamGroupName   = "默认分组"
	defaultTeamImageRemark = "DevLoom 默认开发环境"
)

// NewTeamGroupUserRepo 创建团队分组成员数据访问层 (samber/do 风格)
func NewTeamGroupUserRepo(i *do.Injector) (domain.TeamGroupUserRepo, error) {
	return &TeamGroupUserRepo{
		db:     do.MustInvoke[*db.Client](i),
		redis:  do.MustInvoke[*redis.Client](i),
		config: do.MustInvoke[*config.Config](i),
		logger: do.MustInvoke[*slog.Logger](i).With("module", "repo.team_group_user"),
	}, nil
}

// List 获取团队分组列表
func (r *TeamGroupUserRepo) List(ctx context.Context, teamID uuid.UUID) ([]*db.TeamGroup, error) {
	return r.db.TeamGroup.Query().
		Where(teamgroup.TeamIDEQ(teamID)).
		WithMembers(
			func(uq *db.UserQuery) {
				uq.Where(user.DeletedAtIsNil())
				uq.Order(user.ByCreatedAt(sql.OrderDesc()))
			},
		).
		Order(teamgroup.ByCreatedAt(sql.OrderDesc())).
		All(ctx)
}

// Get 获取团队分组
func (r *TeamGroupUserRepo) Get(ctx context.Context, groupID uuid.UUID) (*db.TeamGroup, error) {
	return r.db.TeamGroup.Get(ctx, groupID)
}

// Create 创建团队分组
func (r *TeamGroupUserRepo) Create(ctx context.Context, teamID uuid.UUID, req *domain.AddTeamGroupReq) (*db.TeamGroup, error) {
	return r.db.TeamGroup.Create().
		SetTeamID(teamID).
		SetName(req.Name).
		Save(ctx)
}

func (r *TeamGroupUserRepo) CreateUsers(ctx context.Context, teamID uuid.UUID, req *domain.AddTeamUserReq) ([]*db.User, error) {
	if err := r.checkTeamMemberLimit(ctx, teamID, req.Emails); err != nil {
		return nil, err
	}
	users := make([]*db.User, 0, len(req.Emails))
	for _, emailAddr := range req.Emails {
		account, err := r.db.User.Query().Where(user.EmailEQ(emailAddr)).First(ctx)
		if err != nil && !db.IsNotFound(err) {
			return nil, err
		}
		if account == nil {
			account, err = r.db.User.Create().
				SetID(uuid.New()).
				SetName(emailAddr).
				SetEmail(emailAddr).
				SetStatus(consts.UserStatusActive).
				SetPassword("").
				SetRole(consts.UserRoleSubAccount).
				Save(ctx)
			if err != nil {
				return nil, err
			}
		}
		exists, err := r.db.TeamMember.Query().Where(
			teammember.TeamIDEQ(teamID),
			teammember.UserIDEQ(account.ID),
		).Exist(ctx)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		if _, err = r.db.TeamMember.Create().
			SetID(uuid.New()).
			SetTeamID(teamID).
			SetUserID(account.ID).
			SetRole(consts.TeamMemberRoleUser).
			Save(ctx); err != nil {
			return nil, err
		}
		users = append(users, account)
	}
	return users, nil
}

func (r *TeamGroupUserRepo) CreateUsersWithPassword(ctx context.Context, teamID uuid.UUID, req *domain.AddTeamUserWithPasswordReq) ([]*db.User, error) {
	if err := r.checkTeamMemberLimit(ctx, teamID, req.Emails); err != nil {
		return nil, err
	}
	users := make([]*db.User, 0, len(req.Emails))
	for _, emailAddr := range req.Emails {
		account, err := r.db.User.Query().Where(user.EmailEQ(emailAddr)).First(ctx)
		if err != nil && !db.IsNotFound(err) {
			return nil, err
		}
		password := req.Passwords[emailAddr]
		if account == nil {
			hashedPassword, err := crypto.HashPassword(password)
			if err != nil {
				return nil, errcode.ErrPasswordHashFailed
			}
			account, err = r.db.User.Create().
				SetID(uuid.New()).
				SetName(emailAddr).
				SetEmail(emailAddr).
				SetStatus(consts.UserStatusActive).
				SetPassword(hashedPassword).
				SetRole(consts.UserRoleSubAccount).
				Save(ctx)
			if err != nil {
				return nil, err
			}
		} else if account.Password == "" {
			hashedPassword, err := crypto.HashPassword(password)
			if err != nil {
				return nil, errcode.ErrPasswordHashFailed
			}
			account, err = r.db.User.UpdateOneID(account.ID).SetPassword(hashedPassword).Save(ctx)
			if err != nil {
				return nil, err
			}
		}
		exists, err := r.db.TeamMember.Query().Where(
			teammember.TeamIDEQ(teamID),
			teammember.UserIDEQ(account.ID),
		).Exist(ctx)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		if _, err = r.db.TeamMember.Create().
			SetID(uuid.New()).
			SetTeamID(teamID).
			SetUserID(account.ID).
			SetRole(consts.TeamMemberRoleUser).
			Save(ctx); err != nil {
			return nil, err
		}
		users = append(users, account)
	}
	return users, nil
}

func (r *TeamGroupUserRepo) ResetPassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	hashedPassword, err := crypto.HashPassword(newPassword)
	if err != nil {
		return errcode.ErrPasswordHashFailed
	}
	return r.db.User.UpdateOneID(userID).SetPassword(hashedPassword).Exec(ctx)
}

func (r *TeamGroupUserRepo) CreateAdmin(ctx context.Context, teamID uuid.UUID, req *domain.AddTeamAdminReq) (*db.User, error) {
	if err := r.checkTeamMemberLimit(ctx, teamID, []string{req.Email}); err != nil {
		return nil, err
	}
	account, err := r.db.User.Query().Where(user.EmailEQ(req.Email)).First(ctx)
	if err != nil && !db.IsNotFound(err) {
		return nil, err
	}
	if account == nil {
		account, err = r.db.User.Create().
			SetID(uuid.New()).
			SetName(req.Name).
			SetEmail(req.Email).
			SetStatus(consts.UserStatusActive).
			SetPassword("").
			SetRole(consts.UserRoleIndividual).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	}
	exists, err := r.db.TeamMember.Query().Where(
		teammember.TeamIDEQ(teamID),
		teammember.UserIDEQ(account.ID),
	).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errcode.ErrUserAlreadyExists
	}
	if _, err = r.db.TeamMember.Create().
		SetID(uuid.New()).
		SetTeamID(teamID).
		SetUserID(account.ID).
		SetRole(consts.TeamMemberRoleAdmin).
		Save(ctx); err != nil {
		return nil, err
	}
	return account, nil
}

func (r *TeamGroupUserRepo) checkTeamMemberLimit(ctx context.Context, teamID uuid.UUID, emails []string) error {
	team, err := r.db.Team.Get(ctx, teamID)
	if err != nil {
		return err
	}
	if team.MemberLimit <= 0 {
		return nil
	}
	existingCount, err := r.db.TeamMember.Query().Where(teammember.TeamIDEQ(teamID)).Count(ctx)
	if err != nil {
		return err
	}
	addCount, err := r.countNewTeamMembers(ctx, teamID, emails)
	if err != nil {
		return err
	}
	if existingCount+addCount > team.MemberLimit {
		return errcode.ErrTeamMemberLimitExceeded
	}
	return nil
}

func (r *TeamGroupUserRepo) countNewTeamMembers(ctx context.Context, teamID uuid.UUID, emails []string) (int, error) {
	seen := make(map[string]struct{}, len(emails))
	count := 0

	for _, emailAddr := range emails {
		if _, ok := seen[emailAddr]; ok {
			continue
		}
		seen[emailAddr] = struct{}{}

		existingUser, err := r.db.User.Query().Where(user.EmailEQ(emailAddr)).First(ctx)
		if err != nil {
			if db.IsNotFound(err) {
				count++
				continue
			}
			return 0, err
		}
		exists, err := r.db.TeamMember.Query().
			Where(
				teammember.TeamIDEQ(teamID),
				teammember.UserIDEQ(existingUser.ID),
			).
			Exist(ctx)
		if err != nil {
			return 0, err
		}
		if !exists {
			count++
		}
	}
	return count, nil
}

// Update 更新团队分组
func (r *TeamGroupUserRepo) Update(ctx context.Context, req *domain.UpdateTeamGroupReq) (*db.TeamGroup, error) {
	return r.db.TeamGroup.UpdateOneID(req.GroupID).
		SetName(req.Name).
		Save(ctx)
}

// Delete 删除团队分组
func (r *TeamGroupUserRepo) Delete(ctx context.Context, teamID, groupID uuid.UUID) error {
	err := r.db.TeamGroup.DeleteOneID(groupID).Exec(ctx)
	return err
}

// ListGroupUsers 获取团队组成员列表
func (r *TeamGroupUserRepo) ListGroupUsers(ctx context.Context, groupID uuid.UUID) ([]*db.TeamGroupMember, error) {
	return r.db.TeamGroupMember.Query().
		Where(
			teamgroupmember.GroupIDEQ(groupID),
			teamgroupmember.HasUserWith(user.DeletedAtIsNil()),
		).
		WithUser().
		All(ctx)
}

// ModifyGroupUsers 添加团队组成员
func (r *TeamGroupUserRepo) ModifyGroupUsers(ctx context.Context, groupID uuid.UUID, userIDs []uuid.UUID) ([]*db.TeamGroupMember, error) {
	var members []*db.TeamGroupMember

	for _, userID := range userIDs {
		// 检查是否已在组中
		existing, err := r.db.TeamGroupMember.Query().
			Where(
				teamgroupmember.GroupIDEQ(groupID),
				teamgroupmember.UserIDEQ(userID),
			).First(ctx)
		if err == nil && existing != nil {
			members = append(members, existing)
			continue
		}

		// 添加到组
		member, err := r.db.TeamGroupMember.Create().
			SetGroupID(groupID).
			SetUserID(userID).
			Save(ctx)
		if err != nil {
			r.logger.ErrorContext(ctx, "add user to group failed", "error", err, "user_id", userID)
			continue
		}
		members = append(members, member)
	}
	return members, nil
}

// DeleteGroupUser 删除团队组成员
func (r *TeamGroupUserRepo) DeleteGroupUser(ctx context.Context, groupID, userID uuid.UUID) error {
	_, err := r.db.TeamGroupMember.Delete().
		Where(
			teamgroupmember.GroupIDEQ(groupID),
			teamgroupmember.UserIDEQ(userID),
		).Exec(ctx)
	return err
}

// Login 团队用户登录
func (r *TeamGroupUserRepo) Login(ctx context.Context, req *domain.TeamLoginReq) (*db.User, error) {
	usr, err := r.db.User.Query().
		WithTeams().
		Where(user.Email(req.Email)).
		Where(user.Role(consts.UserRoleEnterprise)).
		First(ctx)
	if err != nil {
		return nil, errcode.ErrLoginFailed.Wrap(err)
	}

	err = crypto.VerifyPassword(usr.Password, req.Password)
	if err != nil {
		r.logger.Error("invalid password", "email", req.Email, "error", err)
		return nil, errcode.ErrLoginFailed
	}
	return usr, nil
}

// MemberList 获取团队成员列表
func (r *TeamGroupUserRepo) MemberList(ctx context.Context, teamID uuid.UUID, role consts.TeamMemberRole) ([]*db.TeamMember, error) {
	query := r.db.TeamMember.Query().
		Where(
			teammember.TeamIDEQ(teamID),
			teammember.HasUserWith(user.DeletedAtIsNil()),
		).
		WithUser()

	if role != "" {
		query = query.Where(teammember.RoleEQ(role))
	}

	return query.All(ctx)
}

// ChangePassword 修改密码
func (r *TeamGroupUserRepo) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	uu, err := r.db.User.Query().Where(user.IDEQ(userID)).First(ctx)
	if err != nil {
		return err
	}

	if uu.Password != "" {
		err = crypto.VerifyPassword(uu.Password, currentPassword)
		if err != nil {
			return errcode.ErrInvalidPassword
		}
	}

	hashedNewPassword, err := crypto.HashPassword(newPassword)
	if err != nil {
		return errcode.ErrPasswordHashFailed
	}

	return r.db.User.UpdateOneID(userID).SetPassword(hashedNewPassword).Exec(ctx)
}

// GetTeam 获取团队
func (r *TeamGroupUserRepo) GetTeam(ctx context.Context, teamID uuid.UUID) (*db.Team, error) {
	return r.db.Team.Get(ctx, teamID)
}

// UpdateUser 更新团队用户信息
func (r *TeamGroupUserRepo) UpdateUser(ctx context.Context, userID uuid.UUID, req *domain.UpdateTeamUserReq) (*db.User, error) {
	update := r.db.User.UpdateOneID(userID)
	if req.Name != nil {
		update = update.SetName(*req.Name)
	}
	if req.IsBlocked != nil {
		update = update.SetIsBlocked(*req.IsBlocked)
	}
	return update.Save(ctx)
}

func (r *TeamGroupUserRepo) DeleteUser(ctx context.Context, teamID, userID uuid.UUID) error {
	exists, err := r.db.TeamMember.Query().
		Where(
			teammember.TeamIDEQ(teamID),
			teammember.UserIDEQ(userID),
			teammember.HasUserWith(user.DeletedAtIsNil()),
		).
		Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return errcode.ErrNotFound
	}
	return r.db.User.DeleteOneID(userID).Exec(ctx)
}

// GetMembersByIDs 根据用户ID列表获取团队成员
func (r *TeamGroupUserRepo) GetMembersByIDs(ctx context.Context, teamID uuid.UUID, userIDs []uuid.UUID) ([]*db.TeamMember, error) {
	return r.db.TeamMember.Query().
		Where(
			teammember.TeamIDEQ(teamID),
			teammember.UserIDIn(userIDs...),
		).
		WithUser().
		All(ctx)
}

// GetMember 获取团队成员
func (r *TeamGroupUserRepo) GetMember(ctx context.Context, teamID, userID uuid.UUID) (*db.TeamMember, error) {
	return r.db.TeamMember.Query().
		Where(
			teammember.TeamIDEQ(teamID),
			teammember.UserIDEQ(userID),
			teammember.HasUserWith(user.DeletedAtIsNil()),
		).
		WithUser().
		First(ctx)
}

// InitTeam 初始化团队：创建管理员、普通成员、团队和默认资源。
func (r *TeamGroupUserRepo) InitTeam(ctx context.Context, email string, name string, password string, imageName string) (*domain.InitTeamResult, error) {
	var result *domain.InitTeamResult
	err := entx.WithTx2(ctx, r.db, func(tx *db.Tx) error {
		hashedPassword, err := crypto.HashPassword(password)
		if err != nil {
			return err
		}

		existingUser, err := tx.User.Query().
			Where(user.EmailEQ(email), user.RoleEQ(consts.UserRoleEnterprise)).
			First(ctx)
		if err != nil {
			if !db.IsNotFound(err) {
				return err
			}
		}

		var initUser *db.User
		var initTeam *db.Team
		if existingUser == nil {
			initUser, err = tx.User.Create().
				SetID(uuid.New()).
				SetName(name).
				SetEmail(email).
				SetStatus(consts.UserStatusActive).
				SetPassword(hashedPassword).
				SetRole(consts.UserRoleEnterprise).
				Save(ctx)
			if err != nil {
				return err
			}

			// 创建团队
			initTeam, err = tx.Team.Create().
				SetID(uuid.New()).
				SetName(name).
				SetMemberLimit(5).
				Save(ctx)
			if err != nil {
				return err
			}

			// 将用户添加为团队管理员
			if _, err = tx.TeamMember.Create().
				SetID(uuid.New()).
				SetTeamID(initTeam.ID).
				SetUserID(initUser.ID).
				SetRole(consts.TeamMemberRoleAdmin).
				Save(ctx); err != nil {
				return err
			}
		} else {
			initUser = existingUser
			member, err := tx.TeamMember.Query().
				Where(teammember.UserIDEQ(initUser.ID)).
				First(ctx)
			if err != nil {
				if db.IsNotFound(err) {
					return fmt.Errorf("init team user %s has no team member", email)
				}
				return err
			}
			initTeam, err = tx.Team.Get(ctx, member.TeamID)
			if err != nil {
				return err
			}
		}

		defaultGroup, err := r.ensureDefaultTeamGroup(ctx, tx, initTeam.ID)
		if err != nil {
			return err
		}
		if err := r.ensureInitTeamMember(ctx, tx, initTeam.ID, email, name, hashedPassword, defaultGroup.ID); err != nil {
			return err
		}
		// Provision team 私有的 bare skill_repo / plugin_repo,供团队管理员
		// 后续上传 zip 时挂载子 skill。幂等(已存在则跳过)。
		if _, _, err := agentresource.EnsureTeamBareReposTx(ctx, tx, initTeam.ID, initUser.ID); err != nil {
			return err
		}
		if err := r.initTeamImage(ctx, tx, initTeam.ID, defaultGroup.ID, initUser.ID, imageName); err != nil {
			return err
		}
		result = &domain.InitTeamResult{TeamID: initTeam.ID, UserID: initUser.ID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *TeamGroupUserRepo) ensureInitTeamMember(ctx context.Context, tx *db.Tx, teamID uuid.UUID, email, name, hashedPassword string, groupID uuid.UUID) error {
	memberUser, err := tx.User.Query().
		Where(user.EmailEQ(email), user.RoleEQ(consts.UserRoleSubAccount)).
		First(ctx)
	if err != nil {
		if !db.IsNotFound(err) {
			return err
		}
		memberUser, err = tx.User.Create().
			SetID(uuid.New()).
			SetName(name).
			SetEmail(email).
			SetStatus(consts.UserStatusActive).
			SetPassword(hashedPassword).
			SetRole(consts.UserRoleSubAccount).
			Save(ctx)
		if err != nil {
			return err
		}
	}

	exists, err := tx.TeamMember.Query().
		Where(teammember.TeamIDEQ(teamID), teammember.UserIDEQ(memberUser.ID)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := tx.TeamMember.Create().
			SetID(uuid.New()).
			SetTeamID(teamID).
			SetUserID(memberUser.ID).
			SetRole(consts.TeamMemberRoleUser).
			Save(ctx); err != nil {
			return err
		}
	}

	exists, err = tx.TeamGroupMember.Query().
		Where(teamgroupmember.GroupIDEQ(groupID), teamgroupmember.UserIDEQ(memberUser.ID)).
		Exist(ctx)
	if err != nil || exists {
		return err
	}
	return tx.TeamGroupMember.Create().
		SetID(uuid.New()).
		SetGroupID(groupID).
		SetUserID(memberUser.ID).
		Exec(ctx)
}

func (r *TeamGroupUserRepo) ensureDefaultTeamGroup(ctx context.Context, tx *db.Tx, teamID uuid.UUID) (*db.TeamGroup, error) {
	return ensureDefaultTeamGroupTx(ctx, tx, teamID)
}

func ensureDefaultGroupIDs(ctx context.Context, tx *db.Tx, teamID uuid.UUID, groupIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(groupIDs) > 0 {
		return groupIDs, nil
	}
	group, err := ensureDefaultTeamGroupTx(ctx, tx, teamID)
	if err != nil {
		return nil, err
	}
	return []uuid.UUID{group.ID}, nil
}

func ensureDefaultTeamGroupTx(ctx context.Context, tx *db.Tx, teamID uuid.UUID) (*db.TeamGroup, error) {
	group, err := tx.TeamGroup.Query().
		Where(teamgroup.TeamIDEQ(teamID), teamgroup.NameEQ(defaultTeamGroupName)).
		First(ctx)
	if err == nil {
		return group, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}
	return tx.TeamGroup.Create().
		SetID(uuid.New()).
		SetTeamID(teamID).
		SetName(defaultTeamGroupName).
		Save(ctx)
}

func addDefaultGroupHost(ctx context.Context, tx *db.Tx, teamID uuid.UUID, hostID string) error {
	group, err := ensureDefaultTeamGroupTx(ctx, tx, teamID)
	if err != nil {
		return err
	}
	exists, err := tx.TeamGroupHost.Query().
		Where(teamgrouphost.GroupIDEQ(group.ID), teamgrouphost.HostIDEQ(hostID)).
		Exist(ctx)
	if err != nil || exists {
		return err
	}
	return tx.TeamGroupHost.Create().
		SetID(uuid.New()).
		SetGroupID(group.ID).
		SetHostID(hostID).
		Exec(ctx)
}

func (r *TeamGroupUserRepo) initTeamImage(ctx context.Context, tx *db.Tx, teamID, groupID, userID uuid.UUID, imageName string) error {
	if imageName == "" {
		return nil
	}
	managedImages, err := tx.Image.Query().
		Where(image.UserIDEQ(userID), image.RemarkEQ(defaultTeamImageRemark)).
		Order(image.ByCreatedAt(sql.OrderDesc())).
		All(ctx)
	if err != nil {
		return err
	}
	img, err := tx.Image.Query().
		Where(image.UserIDEQ(userID), image.NameEQ(imageName)).
		First(ctx)
	if err != nil {
		if !db.IsNotFound(err) {
			return err
		}
		if len(managedImages) > 0 {
			img, err = tx.Image.UpdateOneID(managedImages[0].ID).
				SetName(imageName).
				SetRemark(defaultTeamImageRemark).
				Save(ctx)
		} else {
			img, err = tx.Image.Create().
				SetID(uuid.New()).
				SetUserID(userID).
				SetName(imageName).
				SetRemark(defaultTeamImageRemark).
				Save(ctx)
		}
		if err != nil {
			return err
		}
	}

	staleManagedIDs := make([]uuid.UUID, 0, len(managedImages))
	for _, managed := range managedImages {
		if managed.ID != img.ID {
			staleManagedIDs = append(staleManagedIDs, managed.ID)
		}
	}
	if len(staleManagedIDs) > 0 {
		if _, err := tx.TeamGroupImage.Delete().
			Where(teamgroupimage.GroupIDEQ(groupID), teamgroupimage.ImageIDIn(staleManagedIDs...)).
			Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.TeamImage.Delete().
			Where(teamimage.TeamIDEQ(teamID), teamimage.ImageIDIn(staleManagedIDs...)).
			Exec(ctx); err != nil {
			return err
		}
	}

	exists, err := tx.TeamImage.Query().
		Where(teamimage.TeamIDEQ(teamID), teamimage.ImageIDEQ(img.ID)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if err := tx.TeamImage.Create().
			SetID(uuid.New()).
			SetTeamID(teamID).
			SetImageID(img.ID).
			Exec(ctx); err != nil {
			return err
		}
	}
	groupImageExists, err := tx.TeamGroupImage.Query().
		Where(teamgroupimage.GroupIDEQ(groupID), teamgroupimage.ImageIDEQ(img.ID)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if groupImageExists {
		return nil
	}
	return tx.TeamGroupImage.Create().
		SetID(uuid.New()).
		SetGroupID(groupID).
		SetImageID(img.ID).
		Exec(ctx)
}
