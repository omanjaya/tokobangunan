package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

const generatedPasswordLength = 16

// UserAccountService - business logic user management.
type UserAccountService struct {
	repo *repo.UserAccountRepo
}

func NewUserAccountService(r *repo.UserAccountRepo) *UserAccountService {
	return &UserAccountService{repo: r}
}

// CreateResult - hasil Create yang menyertakan plaintext password (sekali pakai).
type CreateResult struct {
	User              *domain.UserAccount
	PlaintextPassword string
}

func (s *UserAccountService) List(ctx context.Context, f repo.ListUserFilter) (PageResult[domain.UserAccount], error) {
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return PageResult[domain.UserAccount]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

func (s *UserAccountService) Get(ctx context.Context, id int64) (*domain.UserAccount, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserAccountService) Create(ctx context.Context, in dto.UserCreateInput) (*CreateResult, error) {
	u := &domain.UserAccount{
		Username:    strings.ToLower(strings.TrimSpace(in.Username)),
		NamaLengkap: strings.TrimSpace(in.NamaLengkap),
		Email:       trimToPtr(in.Email),
		Role:        in.Role,
		GudangID:    in.GudangID,
		IsActive:    in.IsActive,
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	// Role kasir/gudang sebaiknya punya gudang_id, tapi tidak strict (admin bisa
	// terapkan kebijakan sendiri). Owner/admin biasanya gudang_id NULL.
	if u.Role == domain.RoleOwner || u.Role == domain.RoleAdmin {
		u.GudangID = nil
	}

	if existing, err := s.repo.GetByUsername(ctx, u.Username); err == nil && existing != nil {
		return nil, domain.ErrUserUsernameDuplikat
	} else if err != nil && !errors.Is(err, domain.ErrUserAccountNotFound) {
		return nil, err
	}

	plaintext, err := auth.GenerateRandomPassword(generatedPasswordLength)
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}
	hash, err := auth.HashPassword(plaintext)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.Create(ctx, u, hash); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &CreateResult{User: u, PlaintextPassword: plaintext}, nil
}

func (s *UserAccountService) Update(ctx context.Context, id int64, in dto.UserUpdateInput) (*domain.UserAccount, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newUsername := strings.ToLower(strings.TrimSpace(in.Username))
	if newUsername != u.Username {
		if existing, err := s.repo.GetByUsername(ctx, newUsername); err == nil && existing != nil && existing.ID != id {
			return nil, domain.ErrUserUsernameDuplikat
		} else if err != nil && !errors.Is(err, domain.ErrUserAccountNotFound) {
			return nil, err
		}
	}

	u.Username = newUsername
	u.NamaLengkap = strings.TrimSpace(in.NamaLengkap)
	u.Email = trimToPtr(in.Email)
	u.Role = in.Role
	u.GudangID = in.GudangID
	u.IsActive = in.IsActive

	if u.Role == domain.RoleOwner || u.Role == domain.RoleAdmin {
		u.GudangID = nil
	}

	if err := u.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return u, nil
}

// ResetPassword - generate password baru, simpan, kembalikan plaintext sekali.
func (s *UserAccountService) ResetPassword(ctx context.Context, id int64) (string, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return "", err
	}
	plaintext, err := auth.GenerateRandomPassword(generatedPasswordLength)
	if err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	hash, err := auth.HashPassword(plaintext)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	if err := s.repo.UpdatePassword(ctx, id, hash); err != nil {
		return "", err
	}
	return plaintext, nil
}

// ChangePassword - user ganti password sendiri (verify old).
func (s *UserAccountService) ChangePassword(ctx context.Context, id int64, oldPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return domain.ErrUserPasswordLemah
	}
	currentHash, err := s.repo.GetPasswordHash(ctx, id)
	if err != nil {
		return err
	}
	ok, err := auth.VerifyPassword(oldPassword, currentHash)
	if err != nil {
		return fmt.Errorf("verify old password: %w", err)
	}
	if !ok {
		return domain.ErrUserPasswordSalah
	}
	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	return s.repo.UpdatePassword(ctx, id, newHash)
}

func (s *UserAccountService) SetActive(ctx context.Context, id int64, active bool) error {
	return s.repo.SetActive(ctx, id, active)
}
