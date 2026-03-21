package application

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
)

// Re-export sentinel errors from repository for callers.
var (
	ErrTOTPConfigNotFound   = authrepo.ErrTOTPConfigNotFound
	ErrRecoveryCodeNotFound = authrepo.ErrRecoveryCodeNotFound
)

// keep compiler quiet about unused imports
var _ = time.Now
var _ = uuid.New

var (
	ErrTOTPSetupFailed    = errors.New("totp setup failed")
	ErrTOTPDecryptFailed  = errors.New("totp secret decryption failed")
	ErrTOTPEncryptFailed  = errors.New("totp secret encryption failed")
	ErrPasswordMismatch   = errors.New("password does not match")
)

const (
	recoveryCodeCount  = 10
	recoveryCodeLength = 8
	totpIssuer         = "SMPP Server"
)

// TOTPService предоставляет методы для работы с TOTP
type TOTPService struct {
	totpRepo       TOTPRepository
	userRepo       UserRepository
	passwordHasher PasswordHasher
	encryptionKey  []byte // 32-byte AES-256 key
}

// NewTOTPService создает новый сервис TOTP
func NewTOTPService(
	totpRepo TOTPRepository,
	userRepo UserRepository,
	passwordHasher PasswordHasher,
	encryptionKey []byte,
) *TOTPService {
	return &TOTPService{
		totpRepo:       totpRepo,
		userRepo:       userRepo,
		passwordHasher: passwordHasher,
		encryptionKey:  encryptionKey,
	}
}

// SetupTOTP настраивает TOTP для пользователя
func (s *TOTPService) SetupTOTP(ctx context.Context, userID uuid.UUID, accountName string) (string, string, []string, error) {
	// Проверяем, не включен ли уже TOTP
	config, err := s.totpRepo.GetTOTPConfig(ctx, userID)
	if err != nil && err != authrepo.ErrTOTPConfigNotFound {
		return "", "", nil, err
	}
	if config != nil && config.IsEnabled() {
		return "", "", nil, domain.ErrTOTPAlreadyEnabled
	}

	// Генерируем TOTP секрет
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: accountName,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("%w: %v", ErrTOTPSetupFailed, err)
	}

	secret := key.Secret()
	qrURL := key.URL()

	// Шифруем секрет
	encryptedSecret, err := s.encrypt([]byte(secret), s.encryptionKey)
	if err != nil {
		return "", "", nil, fmt.Errorf("%w: %v", ErrTOTPEncryptFailed, err)
	}

	// Сохраняем зашифрованный секрет
	if err := s.totpRepo.SaveTOTPSecret(ctx, userID, encryptedSecret); err != nil {
		return "", "", nil, err
	}

	// Генерируем коды восстановления
	recoveryCodes, err := s.generateRecoveryCodes()
	if err != nil {
		return "", "", nil, err
	}

	// Удаляем старые коды восстановления
	if err := s.totpRepo.DeleteRecoveryCodes(ctx, userID); err != nil {
		return "", "", nil, err
	}

	// Хешируем и сохраняем коды восстановления
	now := time.Now()
	domainCodes := make([]*domain.TOTPRecoveryCode, len(recoveryCodes))
	for i, code := range recoveryCodes {
		domainCodes[i] = &domain.TOTPRecoveryCode{
			ID:        uuid.New(),
			UserID:    userID,
			CodeHash:  s.hashRecoveryCode(code),
			Used:      false,
			CreatedAt: now,
		}
	}

	if err := s.totpRepo.SaveRecoveryCodes(ctx, domainCodes); err != nil {
		return "", "", nil, err
	}

	return secret, qrURL, recoveryCodes, nil
}

// VerifyAndEnable верифицирует TOTP код и включает TOTP
func (s *TOTPService) VerifyAndEnable(ctx context.Context, userID uuid.UUID, code string) error {
	// Получаем конфигурацию TOTP
	config, err := s.totpRepo.GetTOTPConfig(ctx, userID)
	if err != nil {
		return err
	}

	if config.IsEnabled() {
		return domain.ErrTOTPAlreadyEnabled
	}

	if config.SecretEncrypted == nil {
		return domain.ErrTOTPNotEnabled
	}

	// Дешифруем секрет
	secret, err := s.decrypt(config.SecretEncrypted, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTOTPDecryptFailed, err)
	}

	// Валидируем код
	valid := totp.Validate(code, string(secret))
	if !valid {
		return domain.ErrInvalidTOTPCode
	}

	// Включаем TOTP
	return s.totpRepo.EnableTOTP(ctx, userID)
}

// ValidateCode валидирует TOTP код
func (s *TOTPService) ValidateCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	// Получаем конфигурацию TOTP
	config, err := s.totpRepo.GetTOTPConfig(ctx, userID)
	if err != nil {
		return false, err
	}

	if !config.IsEnabled() {
		return false, domain.ErrTOTPNotEnabled
	}

	// Дешифруем секрет
	secret, err := s.decrypt(config.SecretEncrypted, s.encryptionKey)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrTOTPDecryptFailed, err)
	}

	// Валидируем код
	valid := totp.Validate(code, string(secret))
	return valid, nil
}

// ValidateRecoveryCode валидирует код восстановления
func (s *TOTPService) ValidateRecoveryCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	codeHash := s.hashRecoveryCode(code)

	err := s.totpRepo.UseRecoveryCode(ctx, userID, codeHash)
	if err != nil {
		if err == authrepo.ErrRecoveryCodeNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Disable отключает TOTP для пользователя (требует пароль)
func (s *TOTPService) Disable(ctx context.Context, userID uuid.UUID, password string) error {
	// Получаем пользователя для проверки пароля
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Проверяем пароль
	if !s.passwordHasher.CheckPassword(password, user.PasswordHash) {
		return ErrPasswordMismatch
	}

	// Отключаем TOTP
	if err := s.totpRepo.DisableTOTP(ctx, userID); err != nil {
		return err
	}

	// Удаляем коды восстановления
	return s.totpRepo.DeleteRecoveryCodes(ctx, userID)
}

// encrypt шифрует данные с использованием AES-256-GCM
func (s *TOTPService) encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt дешифрует данные с использованием AES-256-GCM
func (s *TOTPService) decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// generateRecoveryCodes генерирует случайные коды восстановления
func (s *TOTPService) generateRecoveryCodes() ([]string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	codes := make([]string, recoveryCodeCount)

	for i := 0; i < recoveryCodeCount; i++ {
		code := make([]byte, recoveryCodeLength)
		randomBytes := make([]byte, recoveryCodeLength)
		if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
			return nil, err
		}
		for j := 0; j < recoveryCodeLength; j++ {
			code[j] = charset[int(randomBytes[j])%len(charset)]
		}
		codes[i] = string(code)
	}

	return codes, nil
}

// hashRecoveryCode хеширует код восстановления с помощью SHA256
func (s *TOTPService) hashRecoveryCode(code string) string {
	hash := sha256.Sum256([]byte(code))
	return hex.EncodeToString(hash[:])
}
