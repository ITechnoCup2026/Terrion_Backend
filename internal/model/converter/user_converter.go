package converter

import (
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
)

func UserToResponse(user *entity.AppUser) *model.UserResponse {
	return &model.UserResponse{
		ID:            user.ID,
		Role:          user.Role,
		CooperativeID: user.CooperativeID,
		FullName:      user.FullName,
		Organisation:  user.Organisation,
	}
}
