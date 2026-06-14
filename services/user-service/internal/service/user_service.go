package service

import (
	"context"

	"github.com/auction-system/user-service/internal/dto"
	"github.com/auction-system/user-service/internal/repository"
)

type UserService struct {
	users *repository.UserRepository
}

func NewUserService() *UserService {
	return &UserService{users: repository.NewUserRepository()}
}

func (s *UserService) GetMe(ctx context.Context, userID string) (*dto.UserResponse, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := toUserResponse(u)
	return &resp, nil
}

func (s *UserService) UpdateMe(ctx context.Context, userID string, req dto.UpdateProfileRequest) (*dto.UserResponse, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	fn, ln := u.FirstName, u.LastName
	if req.FirstName != "" {
		fn = req.FirstName
	}
	if req.LastName != "" {
		ln = req.LastName
	}
	if err := s.users.UpdateProfile(ctx, userID, fn, ln); err != nil {
		return nil, err
	}
	u.FirstName, u.LastName = fn, ln
	resp := toUserResponse(u)
	return &resp, nil
}
