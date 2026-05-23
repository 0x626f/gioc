package gioc

// Token is the unique string identifier for a provider within a module.
// It is used as the key for dependency lookup and injection matching.
type Token = string

// Tokenized is implemented by any type that carries a Token identifier.
// Both Module and IProvider satisfy this interface, enabling the generic
// DFS traversal to work across both the module graph and provider graph.
type Tokenized interface {
	Token() Token
}

func NewToken(token string) Token {
	return Token(token)
}
