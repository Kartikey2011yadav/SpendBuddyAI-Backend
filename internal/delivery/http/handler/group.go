package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/delivery/http/middleware"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/labstack/echo/v4"
)

type GroupHandler struct {
	groupRepo domain.GroupRepository
	userRepo  domain.UserRepository
}

func NewGroupHandler(groupRepo domain.GroupRepository, userRepo domain.UserRepository) *GroupHandler {
	return &GroupHandler{groupRepo: groupRepo, userRepo: userRepo}
}

// callerID extracts the authenticated user UUID from the Echo context.
func callerID(c echo.Context) (uuid.UUID, error) {
	raw, _ := c.Get(middleware.UserIDKey).(string)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, echo.NewHTTPError(http.StatusUnauthorized, "invalid user")
	}
	return id, nil
}

// requireAdmin returns 403 if the caller is not an admin of the group.
func (h *GroupHandler) requireAdmin(c echo.Context, groupID, caller uuid.UUID) error {
	role, err := h.groupRepo.GetMemberRole(c.Request().Context(), groupID, caller)
	if err != nil {
		return echo.NewHTTPError(http.StatusForbidden, "not a group member")
	}
	if role != domain.RoleAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "admin only")
	}
	return nil
}

// countAdmins returns how many admins a group currently has.
func (h *GroupHandler) countAdmins(c echo.Context, groupID uuid.UUID) (int, error) {
	members, err := h.groupRepo.GetMembers(c.Request().Context(), groupID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range members {
		if m.Role == domain.RoleAdmin {
			n++
		}
	}
	return n, nil
}

// ─── Group CRUD ────────────────────────────────────────────────────────────────

// POST /api/v1/groups
func (h *GroupHandler) CreateGroup(c echo.Context) error {
	creatorID, err := callerID(c)
	if err != nil {
		return err
	}

	var body struct {
		Name        string  `json:"name"`
		Description *string `json:"description,omitempty"`
		Currency    string  `json:"currency,omitempty"`
	}
	if err := c.Bind(&body); err != nil || body.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}

	currency := body.Currency
	if currency == "" {
		creator, err := h.userRepo.FindByID(c.Request().Context(), creatorID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "could not fetch user")
		}
		currency = creator.PreferredCurrency
		if currency == "" {
			currency = "USD"
		}
	}
	if !domain.IsSupported(currency) {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported currency: "+currency)
	}

	group := &domain.Group{
		ID:          uuid.New(),
		Name:        body.Name,
		Description: body.Description,
		CreatedBy:   creatorID,
		Currency:    currency,
		CreatedAt:   time.Now(),
	}
	if err := h.groupRepo.Create(c.Request().Context(), group); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create group")
	}

	if err := h.groupRepo.AddMember(c.Request().Context(), &domain.GroupMember{
		GroupID:  group.ID,
		UserID:   creatorID,
		Role:     domain.RoleAdmin,
		JoinedAt: time.Now(),
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add creator as member")
	}

	return c.JSON(http.StatusCreated, group)
}

// GET /api/v1/groups
func (h *GroupHandler) ListGroups(c echo.Context) error {
	uid, err := callerID(c)
	if err != nil {
		return err
	}
	groups, err := h.groupRepo.FindByUserID(c.Request().Context(), uid)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, echo.Map{"groups": groups})
}

// GET /api/v1/groups/:group_id
func (h *GroupHandler) GetGroup(c echo.Context) error {
	uid, err := callerID(c)
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	if ok, err := h.groupRepo.IsMember(c.Request().Context(), groupID, uid); err != nil || !ok {
		return echo.NewHTTPError(http.StatusForbidden, "not a group member")
	}
	group, err := h.groupRepo.FindByID(c.Request().Context(), groupID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "group not found")
	}
	return c.JSON(http.StatusOK, group)
}

// PATCH /api/v1/groups/:group_id  (admin only)
func (h *GroupHandler) UpdateGroup(c echo.Context) error {
	uid, err := callerID(c)
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	if err := h.requireAdmin(c, groupID, uid); err != nil {
		return err
	}

	group, err := h.groupRepo.FindByID(c.Request().Context(), groupID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "group not found")
	}

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		AvatarURL   *string `json:"avatar_url"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if body.Name != nil && *body.Name != "" {
		group.Name = *body.Name
	}
	if body.Description != nil {
		group.Description = body.Description
	}
	if body.AvatarURL != nil {
		group.AvatarURL = body.AvatarURL
	}

	if err := h.groupRepo.UpdateGroup(c.Request().Context(), group); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update group")
	}
	return c.JSON(http.StatusOK, group)
}

// DELETE /api/v1/groups/:group_id  (admin only)
func (h *GroupHandler) DeleteGroup(c echo.Context) error {
	uid, err := callerID(c)
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	if err := h.requireAdmin(c, groupID, uid); err != nil {
		return err
	}
	if err := h.groupRepo.Delete(c.Request().Context(), groupID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete group")
	}
	return c.NoContent(http.StatusNoContent)
}

// ─── Member Management ─────────────────────────────────────────────────────────

// GET /api/v1/groups/:group_id/members
func (h *GroupHandler) ListMembers(c echo.Context) error {
	uid, err := callerID(c)
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	if ok, err := h.groupRepo.IsMember(c.Request().Context(), groupID, uid); err != nil || !ok {
		return echo.NewHTTPError(http.StatusForbidden, "not a group member")
	}
	members, err := h.groupRepo.GetMembersWithDetails(c.Request().Context(), groupID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, echo.Map{"members": members})
}

// POST /api/v1/groups/:group_id/members  (admin only)
func (h *GroupHandler) AddMember(c echo.Context) error {
	uid, err := callerID(c)
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	if err := h.requireAdmin(c, groupID, uid); err != nil {
		return err
	}

	var body struct {
		UserID string `json:"user_id"`
	}
	if err := c.Bind(&body); err != nil || body.UserID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id required")
	}
	targetID, err := uuid.Parse(body.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
	}
	if _, err := h.userRepo.FindByID(c.Request().Context(), targetID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	if err := h.groupRepo.AddMember(c.Request().Context(), &domain.GroupMember{
		GroupID:  groupID,
		UserID:   targetID,
		Role:     domain.RoleMember,
		JoinedAt: time.Now(),
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add member")
	}
	return c.NoContent(http.StatusNoContent)
}

// DELETE /api/v1/groups/:group_id/members/:user_id  (admin only)
func (h *GroupHandler) RemoveMember(c echo.Context) error {
	uid, err := callerID(c)
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	targetID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
	}
	if err := h.requireAdmin(c, groupID, uid); err != nil {
		return err
	}
	if targetID == uid {
		return echo.NewHTTPError(http.StatusBadRequest, "use DELETE /members/me to leave the group")
	}

	// Guard: cannot remove the last admin.
	targetRole, err := h.groupRepo.GetMemberRole(c.Request().Context(), groupID, targetID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "target is not a group member")
	}
	if targetRole == domain.RoleAdmin {
		n, err := h.countAdmins(c, groupID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if n <= 1 {
			return echo.NewHTTPError(http.StatusConflict, "cannot remove the last admin — transfer admin role first")
		}
	}

	if err := h.groupRepo.RemoveMember(c.Request().Context(), groupID, targetID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove member")
	}
	return c.NoContent(http.StatusNoContent)
}

// DELETE /api/v1/groups/:group_id/members/me
func (h *GroupHandler) LeaveGroup(c echo.Context) error {
	uid, err := callerID(c)
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}

	// Guard: last admin cannot leave without transferring the role.
	role, err := h.groupRepo.GetMemberRole(c.Request().Context(), groupID, uid)
	if err != nil {
		return echo.NewHTTPError(http.StatusForbidden, "not a group member")
	}
	if role == domain.RoleAdmin {
		n, err := h.countAdmins(c, groupID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if n <= 1 {
			return echo.NewHTTPError(http.StatusConflict, "cannot leave — you are the last admin, transfer admin role first")
		}
	}

	if err := h.groupRepo.RemoveMember(c.Request().Context(), groupID, uid); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to leave group")
	}
	return c.NoContent(http.StatusNoContent)
}

// PATCH /api/v1/groups/:group_id/members/:user_id  (admin only)
func (h *GroupHandler) UpdateMemberRole(c echo.Context) error {
	uid, err := callerID(c)
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	targetID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
	}
	if err := h.requireAdmin(c, groupID, uid); err != nil {
		return err
	}

	var body struct {
		Role domain.GroupRole `json:"role"`
	}
	if err := c.Bind(&body); err != nil || (body.Role != domain.RoleAdmin && body.Role != domain.RoleMember) {
		return echo.NewHTTPError(http.StatusBadRequest, `role must be "admin" or "member"`)
	}

	// Guard: cannot demote the last admin.
	if body.Role == domain.RoleMember {
		current, err := h.groupRepo.GetMemberRole(c.Request().Context(), groupID, targetID)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "target is not a group member")
		}
		if current == domain.RoleAdmin {
			n, err := h.countAdmins(c, groupID)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			if n <= 1 {
				return echo.NewHTTPError(http.StatusConflict, "cannot demote the last admin")
			}
		}
	}

	if err := h.groupRepo.UpdateMemberRole(c.Request().Context(), groupID, targetID, body.Role); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update role")
	}
	return c.JSON(http.StatusOK, echo.Map{"user_id": targetID, "role": body.Role})
}
