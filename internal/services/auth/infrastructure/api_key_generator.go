package infrastructure

// APIKeyGeneratorImpl реализует интерфейс APIKeyGenerator
type APIKeyGeneratorImpl struct{}

func (g *APIKeyGeneratorImpl) GenerateAPIKey() (string, error) {
	return GenerateAPIKey()
}

func (g *APIKeyGeneratorImpl) GetKeyPrefix(key string) string {
	return GetKeyPrefix(key)
}
