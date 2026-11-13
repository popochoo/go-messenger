package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/popochoo/go-messenger/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	Create(ctx context.Context, username, passwordHash string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByID(ctx context.Context, id int64) (*domain.User, error)
}

type AuthService struct {
	repository UserRepository
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type tokenClaims struct {
	UserID    int64  `json:"user_id"`
	TokenType string `json:"type"`
	jwt.RegisteredClaims
}

func NewAuthService(
	repository UserRepository,
	jwtSecret string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		repository: repository,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (s *AuthService) Register(ctx context.Context, username, password string) (*domain.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	return s.repository.Create(ctx, username, string(hash))
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*domain.TokenPair, error) {
	user, err := s.repository.GetByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidPassword
	}

	return s.generateTokenPair(user.ID)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	userID, err := s.ParseToken(refreshToken, "refresh")
	if err != nil {
		return nil, fmt.Errorf("error parse refresh token: %w", err)
	}

	if _, err := s.repository.GetByID(ctx, userID); err != nil {
		return nil, domain.ErrInvalidToken
	}

	return s.generateTokenPair(userID)
}

func (s *AuthService) ParseToken(tokenStr, expectedType string) (int64, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &tokenClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, domain.ErrInvalidToken
	}

	claims, ok := token.Claims.(*tokenClaims)
	if !ok || claims.TokenType != expectedType {
		return 0, domain.ErrInvalidToken
	}

	return claims.UserID, nil
}

func (s *AuthService) generateTokenPair(userID int64) (*domain.TokenPair, error) {
	accessToken, err := s.createToken(userID, "access", s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("error generate access token: %w", err)
	}

	refreshToken, err := s.createToken(userID, "refresh", s.refreshTTL)
	if err != nil {
		return nil, fmt.Errorf("error generate refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) createToken(userID int64, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := tokenClaims{
		UserID:    userID,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(s.jwtSecret)
}
