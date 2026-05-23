package gioc

// Token identifies a provider or module.
type Token = string

// Tokenized exposes a Token.
type Tokenized interface {
	Token() Token
}

// NewToken returns token as a Token.
func NewToken(token string) Token {
	return Token(token)
}
