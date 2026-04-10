package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vanzheng/kodaclaw-community/internal/auth"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type InstanceSelectHandler struct {
	userRepo repository.UserRepository
}

func NewInstanceSelectHandler(userRepo repository.UserRepository) *InstanceSelectHandler {
	return &InstanceSelectHandler{userRepo: userRepo}
}

// ListInstances GET /auth/instances?select_token=xxx
// Returns all instances bound to the GitHub account identified by the select_token.
func (h *InstanceSelectHandler) ListInstances(c *gin.Context) {
	selectToken := c.Query("select_token")
	if selectToken == "" {
		middleware.RespondError(c, http.StatusBadRequest, "MISSING_TOKEN", "select_token is required")
		return
	}

	githubID, err := auth.ValidateSelectToken(c.Request.Context(), selectToken)
	if err != nil {
		middleware.RespondError(c, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
		return
	}

	users, err := h.userRepo.GetByGitHubIDs(c.Request.Context(), githubID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch instances")
		return
	}

	type instanceInfo struct {
		Username       string  `json:"username"`
		DisplayName    *string `json:"display_name"`
		GitHubUsername *string `json:"github_username"`
		AvatarURL      *string `json:"avatar_url"`
		CreatedAt      string  `json:"created_at"`
	}

	instances := make([]instanceInfo, 0, len(users))
	for _, u := range users {
		instances = append(instances, instanceInfo{
			Username:       u.Username,
			DisplayName:    u.DisplayName,
			GitHubUsername: u.GitHubUsername,
			AvatarURL:      u.AvatarURL,
			CreatedAt:      u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"instances": instances})
}

// SelectInstance POST /auth/instance/select
// Body: { "select_token": "xxx", "username": "nietzsche" }
func (h *InstanceSelectHandler) SelectInstance(c *gin.Context) {
	var req struct {
		SelectToken string `json:"select_token" binding:"required"`
		Username    string `json:"username" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	githubID, err := auth.ConsumeSelectToken(c.Request.Context(), req.SelectToken)
	if err != nil {
		middleware.RespondError(c, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
		return
	}

	users, err := h.userRepo.GetByGitHubIDs(c.Request.Context(), githubID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch instances")
		return
	}

	// Find the selected instance
	var selected *model.User
	for i := range users {
		if users[i].Username == req.Username {
			selected = &users[i]
			break
		}
	}
	if selected == nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Instance not found")
		return
	}

	// Issue JWT
	jwtToken, err := auth.GenerateToken(selected.ID.String(), selected.Username, selected.IsAdmin)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate token")
		return
	}

	c.JSON(http.StatusOK, gin.H{"jwt": jwtToken})
}
