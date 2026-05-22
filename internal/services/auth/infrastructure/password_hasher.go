package infrastructure

// PasswordHasherImpl реализует интерфейс PasswordHasher
type PasswordHasherImpl struct{}

func (p *PasswordHasherImpl) HashPassword(password string) (string, error) {
	return HashPassword(password)
}

func (p *PasswordHasherImpl) CheckPassword(password, hash string) bool {
	return CheckPassword(password, hash)
}
